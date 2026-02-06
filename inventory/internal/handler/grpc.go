package handler

import (
	"context"
	"fmt"
	"time"

	"auto_grocery/inventory/internal/store"
	pb "auto_grocery/inventory/proto" // Now contains BOTH Inventory and Pricing definitions
)

// InventoryHandler implements the gRPC server interface
type InventoryHandler struct {
	pb.UnimplementedInventoryServiceServer
	store         *store.Store
	pricingClient pb.PricingServiceClient // Client to talk to Pricing Service
}

// NewInventoryHandler creates a new handler with Store AND Pricing Client
func NewInventoryHandler(s *store.Store, p pb.PricingServiceClient) *InventoryHandler {
	return &InventoryHandler{
		store:         s,
		pricingClient: p,
	}
}

// ---------------------------------------------------------------------
// 1. CHECK AVAILABILITY
// ---------------------------------------------------------------------
func (h *InventoryHandler) CheckAvailability(ctx context.Context, req *pb.CheckAvailabilityRequest) (*pb.CheckAvailabilityResponse, error) {
	// 1. Call the Store
	dbItems, err := h.store.GetBatchItems(ctx, req.GetSkus())
	if err != nil {
		return nil, err
	}

	// 2. Convert Store Model -> Proto Model
	protoItems := make(map[string]*pb.ItemDetail)
	for sku, item := range dbItems {
		protoItems[sku] = &pb.ItemDetail{
			Sku:               item.Sku,
			Name:              item.Name,
			AisleType:         item.AisleType,
			QuantityAvailable: item.Quantity,
		}
	}

	return &pb.CheckAvailabilityResponse{
		Items: protoItems,
	}, nil
}

// ---------------------------------------------------------------------
// 2. RESERVE ITEMS (Buying)
// ---------------------------------------------------------------------
func (h *InventoryHandler) ReserveItems(ctx context.Context, req *pb.ReserveItemsRequest) (*pb.ReserveItemsResponse, error) {
	// 1. Call the Store
	reservedItems, err := h.store.ReserveStock(ctx, req.GetItems())
	
	if err != nil {
		return &pb.ReserveItemsResponse{
			OrderId: req.GetOrderId(),
			Items:   nil,
			Success: false,
		}, nil 
	}

	// 2. TODO: TRIGGER ROBOTS (ZeroMQ)
	// h.broker.BroadcastOrder(req.OrderId, reservedItems)

	return &pb.ReserveItemsResponse{
		OrderId: req.GetOrderId(),
		Items:   reservedItems,
		Success: true,
	}, nil
}

// ---------------------------------------------------------------------
// 3. RELEASE ITEMS (Undo)
// ---------------------------------------------------------------------
func (h *InventoryHandler) ReleaseItems(ctx context.Context, req *pb.ReleaseItemsRequest) (*pb.ReleaseItemsResponse, error) {
	err := h.store.ReleaseStock(ctx, req.GetItems())
	if err != nil {
		return &pb.ReleaseItemsResponse{Success: false}, nil
	}
	return &pb.ReleaseItemsResponse{Success: true}, nil
}

// ---------------------------------------------------------------------
// 4. RESTOCK ITEMS (Truck)
// ---------------------------------------------------------------------
func (h *InventoryHandler) RestockItems(ctx context.Context, req *pb.RestockItemsRequest) (*pb.RestockItemsResponse, error) {
	for _, protoItem := range req.GetItems() {
		layout := "2006-01-02" 
		mfd, _ := time.Parse(layout, protoItem.GetMfdDate())
		expiry, _ := time.Parse(layout, protoItem.GetExpiryDate())

		item := store.StockItem{
			Sku:        protoItem.GetSku(),
			Name:       protoItem.GetName(),
			AisleType:  protoItem.GetAisleType(),
			Quantity:   protoItem.GetQuantity(),
			MfdDate:    mfd,
			ExpiryDate: expiry,
		}

		err := h.store.UpsertStock(ctx, item)
		if err != nil {
			fmt.Printf("Error restocking %s: %v\n", item.Sku, err)
			return &pb.RestockItemsResponse{Success: false}, nil
		}
	}

	// TODO: BROADCAST RESTOCK TO ROBOTS
	
	return &pb.RestockItemsResponse{Success: true}, nil
}

// ---------------------------------------------------------------------
// 5. ROBOT REPORTING
// ---------------------------------------------------------------------
func (h *InventoryHandler) ReportJobStatus(ctx context.Context, req *pb.ReportJobStatusRequest) (*pb.ReportJobStatusResponse, error) {
	fmt.Printf("🤖 Robot Report: Order=%s Robot=%s Status=%s\n", 
		req.GetOrderId(), req.GetRobotId(), req.GetStatus())
	return &pb.ReportJobStatusResponse{Success: true}, nil
}

// ---------------------------------------------------------------------
// 6. CHECKOUT (The New Orchestrator)
// ---------------------------------------------------------------------
func (h *InventoryHandler) Checkout(ctx context.Context, req *pb.CheckoutRequest) (*pb.CheckoutResponse, error) {
	fmt.Printf("🛒 Processing Checkout for Order %s...\n", req.GetOrderId())

	// STEP 1: RESERVE STOCK (Local DB)
	// We use the exact same logic as ReserveItems
	reservedItems, err := h.store.ReserveStock(ctx, req.GetItems())
	if err != nil {
		return &pb.CheckoutResponse{
			OrderId: req.GetOrderId(),
			Success: false,
		}, nil
	}

	// If nothing was reserved (out of stock), return early
	if len(reservedItems) == 0 {
		return &pb.CheckoutResponse{
			OrderId: req.GetOrderId(),
			Items:   reservedItems,
			Success: true, // Request succeeded, just got 0 items
			TotalPrice: 0.0,
		}, nil
	}

	// STEP 2: BROADCAST TO ROBOTS
	// (Skipping for now as requested)

	// STEP 3: CALCULATE BILL (Call Pricing Service)
	// Convert Map[string]int32 -> Proto List for Pricing
	var pricingItems []*pb.CartItem
	for sku, qty := range reservedItems {
		pricingItems = append(pricingItems, &pb.CartItem{
			Sku:      sku,
			Quantity: qty,
		})
	}

	// Call the remote Pricing Service
	priceReq := &pb.CalculateBillRequest{Items: pricingItems}
	priceResp, err := h.pricingClient.CalculateBill(ctx, priceReq)
	
	var finalPrice float64 = 0.0
	if err != nil {
		fmt.Printf("⚠️ Pricing Service failed: %v\n", err)
		// We proceed, but price is 0. 
	} else {
		finalPrice = priceResp.GetGrandTotal()
	}

	// STEP 4: RETURN CONSISTENT RESPONSE
	return &pb.CheckoutResponse{
		OrderId:     req.GetOrderId(),
		Items:       reservedItems, // Passes the exact map back
		Success:     true,
		TotalPrice:  finalPrice,
	}, nil
}