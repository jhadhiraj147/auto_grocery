package handler

import (
	"context"
	"log"

	"auto_grocery/pricing/internal/logic"
	"auto_grocery/pricing/internal/store"
	pb "auto_grocery/pricing/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PricingHandler struct {
	pb.UnimplementedPricingServiceServer
	store *store.CatalogStore
}

// NewPricingHandler constructs a pricing gRPC handler.
func NewPricingHandler(s *store.CatalogStore) *PricingHandler {
	return &PricingHandler{store: s}
}

// UpdateStockMetrics recalculates and persists sku prices from stock metrics.
func (h *PricingHandler) UpdateStockMetrics(ctx context.Context, req *pb.UpdateStockMetricsRequest) (*pb.UpdateStockMetricsResponse, error) {
	log.Printf("[pricing] UpdateStockMetrics called updates=%d", len(req.GetUpdates()))
	updatedCount := 0

	// Loop through the repeated list from the Proto
	for _, update := range req.GetUpdates() {
		log.Printf("[pricing] metric input sku=%s qty=%d unit_cost=%.2f", update.GetSku(), update.GetQuantity(), update.GetUnitCost())

		// 1. Calculate the new price using your Logic package
		// This uses the Unit Cost + Scarcity logic
		newPrice := logic.CalculatePrice(update.GetUnitCost(), int(update.GetQuantity()))
		log.Printf("[pricing] metric computed sku=%s new_price=%.2f", update.GetSku(), newPrice)

		// 2. Prepare the item for the Database
		item := store.Item{
			Sku:       update.GetSku(),
			UnitPrice: newPrice,
		}

		// 3. Update the Database
		// UpsertItem handles "Insert if new, Update if exists"
		_, err := h.store.UpsertItem(ctx, item)
		if err != nil {
			// We log the error but CONTINUE so one failure doesn't stop the whole batch
			log.Printf("[pricing] update failed sku=%s err=%v", item.Sku, err)
			continue
		}
		log.Printf("[pricing] update success sku=%s price=%.2f", item.Sku, item.UnitPrice)

		updatedCount++
	}

	log.Printf("[pricing] UpdateStockMetrics complete updated_count=%d", updatedCount)

	return &pb.UpdateStockMetricsResponse{
		Success:      true,
		UpdatedCount: int32(updatedCount),
	}, nil
}

// CalculateBill computes line totals and aggregate amount for a cart payload.
func (h *PricingHandler) CalculateBill(ctx context.Context, req *pb.CalculateBillRequest) (*pb.CalculateBillResponse, error) {
	log.Printf("[pricing] CalculateBill called items=%d", len(req.GetItems()))
	var skus []string
	for _, item := range req.GetItems() {
		skus = append(skus, item.GetSku())
	}
	log.Printf("[pricing] CalculateBill skus=%v", skus)

	itemsMap, err := h.store.GetItemsBySKUs(ctx, skus)
	if err != nil {
		log.Printf("[pricing] CalculateBill fetch prices failed err=%v", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch prices: %v", err)
	}

	var lineItems []*pb.LineItem
	var grandTotal float64

	for _, cartItem := range req.GetItems() {
		dbItem, exists := itemsMap[cartItem.GetSku()]
		if !exists {
			log.Printf("[pricing] CalculateBill missing sku=%s", cartItem.GetSku())
			return nil, status.Errorf(codes.NotFound, "sku %s not found in catalog", cartItem.GetSku())
		}

		totalPrice := dbItem.UnitPrice * float64(cartItem.GetQuantity())
		grandTotal += totalPrice

		lineItems = append(lineItems, &pb.LineItem{
			Sku:        dbItem.Sku,
			UnitPrice:  dbItem.UnitPrice,
			Quantity:   cartItem.GetQuantity(),
			TotalPrice: totalPrice,
		})
		log.Printf("[pricing] line item sku=%s qty=%d unit=%.2f total=%.2f", dbItem.Sku, cartItem.GetQuantity(), dbItem.UnitPrice, totalPrice)
	}
	log.Printf("[pricing] CalculateBill complete grand_total=%.2f", grandTotal)

	return &pb.CalculateBillResponse{
		Items:      lineItems,
		GrandTotal: grandTotal,
	}, nil
}

// CreateItem creates or updates a catalog sku with an explicit unit price.
func (h *PricingHandler) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.CreateItemResponse, error) {
	log.Printf("[pricing] CreateItem called sku=%s unit_price=%.2f", req.GetSku(), req.GetUnitPrice())
	item := store.Item{
		Sku:       req.GetSku(),
		UnitPrice: req.GetUnitPrice(),
	}

	id, err := h.store.UpsertItem(ctx, item)
	if err != nil {
		log.Printf("[pricing] CreateItem failed sku=%s err=%v", req.GetSku(), err)
		return nil, status.Errorf(codes.Internal, "failed to upsert item: %v", err)
	}
	log.Printf("[pricing] CreateItem success sku=%s id=%d", req.GetSku(), id)

	return &pb.CreateItemResponse{
		Id: int32(id),
	}, nil
}

// GetPrice returns the latest stored price for a single sku.
func (h *PricingHandler) GetPrice(ctx context.Context, req *pb.GetPriceRequest) (*pb.GetPriceResponse, error) {
	log.Printf("[pricing] GetPrice called sku=%s", req.GetSku())
	item, err := h.store.GetItem(ctx, req.GetSku())
	if err != nil {
		log.Printf("[pricing] GetPrice failed sku=%s err=%v", req.GetSku(), err)
		return nil, status.Errorf(codes.NotFound, "item with SKU %s not found", req.GetSku())
	}
	log.Printf("[pricing] GetPrice success sku=%s unit_price=%.2f", item.Sku, item.UnitPrice)

	return &pb.GetPriceResponse{
		Id:        int32(item.ID),
		Sku:       item.Sku,
		UnitPrice: item.UnitPrice,
	}, nil
}
