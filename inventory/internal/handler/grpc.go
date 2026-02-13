package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"auto_grocery/inventory/internal/mq"
	"auto_grocery/inventory/internal/store"
	pb "auto_grocery/inventory/proto"
)

type InventoryHandler struct {
	pb.UnimplementedInventoryServiceServer
	store         *store.Store
	memoryStore   *store.MemoryStore
	publisher     *mq.Publisher
	pricingClient pb.PricingServiceClient
}

func NewInventoryHandler(
	s *store.Store,
	ms *store.MemoryStore,
	pub *mq.Publisher,
	p pb.PricingServiceClient,
) *InventoryHandler {
	return &InventoryHandler{
		store:         s,
		memoryStore:   ms,
		publisher:     pub,
		pricingClient: p,
	}
}

func (h *InventoryHandler) AssignRobots(ctx context.Context, req *pb.AssignRequest) (*pb.AssignResponse, error) {
	fmt.Printf("Dispatching Robots for Order %s...\n", req.GetOrderId())

	err := h.memoryStore.SaveOrderItems(ctx, req.GetOrderId(), req.GetItems())
	if err != nil {
		log.Printf("Redis Save Failed: %v", err)
		return nil, err
	}

	skus := make([]string, 0, len(req.GetItems()))
	for sku := range req.GetItems() {
		skus = append(skus, sku)
	}

	dbItems, err := h.store.GetBatchItems(ctx, skus)
	if err != nil {
		log.Printf("DB Lookup Failed: %v", err)
		return nil, err
	}

	robotItems := make(map[string]mq.ItemDetails)
	for sku, qty := range req.GetItems() {
		aisle := "Unknown"
		if item, exists := dbItems[sku]; exists {
			aisle = item.AisleType
		}

		robotItems[sku] = mq.ItemDetails{
			Quantity: qty,
			Aisle:    aisle,
		}
	}

	err = h.publisher.SendRobotCommand(req.GetOrderId(), robotItems)
	if err != nil {
		log.Printf("ZMQ Broadcast Failed: %v", err)
		return nil, err
	}

	return &pb.AssignResponse{
		Success: true,
		Message: "Robots dispatched with aisle info. Order cached.",
	}, nil
}

func (h *InventoryHandler) CheckAvailability(ctx context.Context, req *pb.CheckAvailabilityRequest) (*pb.CheckAvailabilityResponse, error) {
	dbItems, err := h.store.GetBatchItems(ctx, req.GetSkus())
	if err != nil {
		return nil, err
	}

	protoItems := make(map[string]*pb.ItemDetail)
	for sku, item := range dbItems {
		protoItems[sku] = &pb.ItemDetail{
			Sku:               item.SKU,
			Name:              item.Name,
			AisleType:         item.AisleType,
			QuantityAvailable: int32(item.Quantity),
		}
	}

	return &pb.CheckAvailabilityResponse{
		Items: protoItems,
	}, nil
}

func (h *InventoryHandler) ReserveItems(ctx context.Context, req *pb.ReserveItemsRequest) (*pb.ReserveItemsResponse, error) {
	reservedItems, err := h.store.ReserveStock(ctx, req.GetItems())
	if err != nil {
		return &pb.ReserveItemsResponse{
			OrderId: req.GetOrderId(),
			Items:   nil,
			Success: false,
		}, nil
	}

	return &pb.ReserveItemsResponse{
		OrderId: req.GetOrderId(),
		Items:   reservedItems,
		Success: true,
	}, nil
}

func (h *InventoryHandler) ReleaseItems(ctx context.Context, req *pb.ReleaseItemsRequest) (*pb.ReleaseItemsResponse, error) {
	err := h.store.ReleaseStock(ctx, req.GetItems())
	if err != nil {
		return &pb.ReleaseItemsResponse{Success: false}, nil
	}
	return &pb.ReleaseItemsResponse{Success: true}, nil
}

func (h *InventoryHandler) RestockItems(ctx context.Context, req *pb.RestockItemsRequest) (*pb.RestockItemsResponse, error) {
	for _, protoItem := range req.GetItems() {
		layout := "2006-01-02"
		mfd, _ := time.Parse(layout, protoItem.GetMfdDate())
		expiry, _ := time.Parse(layout, protoItem.GetExpiryDate())

		item := store.StockItem{
			SKU:        protoItem.GetSku(),
			Name:       protoItem.GetName(),
			AisleType:  protoItem.GetAisleType(),
			Quantity:   int(protoItem.GetQuantity()),
			UnitCost:   protoItem.GetUnitCost(),
			MfdDate:    mfd,
			ExpiryDate: expiry,
		}

		err := h.store.UpsertStock(ctx, item)
		if err != nil {
			fmt.Printf("Error restocking %s: %v\n", item.SKU, err)
			return &pb.RestockItemsResponse{Success: false}, nil
		}
	}

	return &pb.RestockItemsResponse{Success: true}, nil
}

func (h *InventoryHandler) GetInventoryMetrics(ctx context.Context, req *pb.GetInventoryMetricsRequest) (*pb.GetInventoryMetricsResponse, error) {
	items, err := h.store.GetAllStock(ctx)
	if err != nil {
		return nil, err
	}

	var metrics []*pb.InventoryMetric
	for _, item := range items {
		metrics = append(metrics, &pb.InventoryMetric{
			Sku:      item.SKU,
			Quantity: int32(item.Quantity),
			UnitCost: item.UnitCost,
		})
	}

	return &pb.GetInventoryMetricsResponse{
		Metrics: metrics,
	}, nil
}

func (h *InventoryHandler) ReportJobStatus(ctx context.Context, req *pb.ReportJobStatusRequest) (*pb.ReportJobStatusResponse, error) {
	orderID := req.GetOrderId()
	robotID := req.GetRobotId()

	fmt.Printf("Robot %s reported status for %s: %s\n", robotID, orderID, req.GetStatus())

	count, err := h.memoryStore.IncrementRobotCount(ctx, orderID)
	if err != nil {
		log.Printf("Failed to increment robot count in Redis: %v", err)
		return nil, err
	}

	log.Printf("Order %s progress: %d/5 robots finished", orderID, count)

	if count == 5 {
		log.Printf("All 5 robots finished for Order %s! Finalizing...", orderID)
		go h.finalizeOrder(orderID)
	}

	return &pb.ReportJobStatusResponse{Success: true}, nil
}

func (h *InventoryHandler) finalizeOrder(orderID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get Items from Redis
	items, err := h.memoryStore.GetOrderItems(ctx, orderID)
	if err != nil {
		log.Printf("Finalize Error: Could not find items for %s in Redis: %v", orderID, err)
		return
	}

	// --- DEBUG LOG: Check if Redis actually had the items ---
	log.Printf("🔍 [DEBUG] Finalizing Order %s. Found items in Redis: %v", orderID, items)

	if len(items) == 0 {
		log.Printf("⚠️ [DEBUG] Redis returned 0 items. Pricing cannot be calculated.")
	}

	var pricingItems []*pb.CartItem
	for sku, qty := range items {
		pricingItems = append(pricingItems, &pb.CartItem{
			Sku:      sku,
			Quantity: qty,
		})
	}

	// 2. Ask Pricing Service for the Bill
	priceReq := &pb.CalculateBillRequest{Items: pricingItems}
	priceResp, err := h.pricingClient.CalculateBill(ctx, priceReq)

	finalPrice := 0.0
	if err != nil {
		log.Printf("Pricing Service failed for %s, setting price to 0: %v", orderID, err)
	} else {
		finalPrice = priceResp.GetGrandTotal()
		// --- DEBUG LOG: Check what Pricing Service calculated ---
		log.Printf("💰 [DEBUG] Pricing Service returned: $%0.2f for items %v", finalPrice, items)
	}

	// 3. Update Ordering Service via Webhook
	h.callOrderingWebhook(orderID, finalPrice)

	// 4. Cleanup
	h.memoryStore.DeleteOrderData(ctx, orderID)
	log.Printf("🧹 Cleanup: Removed Redis state for Order %s", orderID)
}

func (h *InventoryHandler) callOrderingWebhook(orderID string, price float64) {
	url := "http://localhost:5050/internal/webhook/update-order"

	payload := map[string]interface{}{
		"order_id":    orderID,
		"status":      "COMPLETED",
		"total_price": price,
	}

	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Printf("Failed to create webhook request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	secret := os.Getenv("INTERNAL_SECRET")
	req.Header.Set("X-Internal-Secret", secret)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("Webhook call failed for %s: %v", orderID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Webhook returned error %d for %s", resp.StatusCode, orderID)
		return
	}

	log.Printf("Webhook Success: Order %s is now officially COMPLETED", orderID)
}