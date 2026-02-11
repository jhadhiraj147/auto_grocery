package handler

import (
	"context"

	"auto_grocery/pricing/internal/store"
	pb "auto_grocery/pricing/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PricingHandler struct {
	pb.UnimplementedPricingServiceServer
	store *store.CatalogStore
}

func NewPricingHandler(s *store.CatalogStore) *PricingHandler {
	return &PricingHandler{store: s}
}

func (h *PricingHandler) CalculateBill(ctx context.Context, req *pb.CalculateBillRequest) (*pb.CalculateBillResponse, error) {
	var skus []string
	for _, item := range req.GetItems() {
		skus = append(skus, item.GetSku())
	}

	itemsMap, err := h.store.GetItemsBySKUs(ctx, skus)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch prices: %v", err)
	}

	var lineItems []*pb.LineItem
	var grandTotal float64

	for _, cartItem := range req.GetItems() {
		dbItem, exists := itemsMap[cartItem.GetSku()]
		if !exists {
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
	}

	return &pb.CalculateBillResponse{
		Items:      lineItems,
		GrandTotal: grandTotal,
	}, nil
}

func (h *PricingHandler) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.CreateItemResponse, error) {
	item := store.Item{
		Sku:       req.GetSku(),
		UnitPrice: req.GetUnitPrice(),
	}

	id, err := h.store.UpsertItem(ctx, item)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to upsert item: %v", err)
	}

	return &pb.CreateItemResponse{
		Id: int32(id),
	}, nil
}


func (h *PricingHandler) GetPrice(ctx context.Context, req *pb.GetPriceRequest) (*pb.GetPriceResponse, error) {
	item, err := h.store.GetItem(ctx, req.GetSku())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "item with SKU %s not found", req.GetSku())
	}

	return &pb.GetPriceResponse{
		Id:        int32(item.ID),
		Sku:       item.Sku,
		UnitPrice: item.UnitPrice,
	}, nil
}