package handler

import (
	"context"
	"auto-grocery/pricing/internal/store"
	pb "auto-grocery/pricing/proto"
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

// CalculateBill implements the bulk calculation logic
func (h *PricingHandler) CalculateBill(ctx context.Context, req *pb.CalculateBillRequest) (*pb.CalculateBillResponse, error) {
	// 1. Extract all SKUs from the request to do a batch lookup
	var skus []string
	for _, item := range req.GetItems() {
		skus = append(skus, item.GetSku())
	}

	// 2. Fetch all items from DB in ONE trip
	itemsMap, err := h.store.GetItemsBySKUs(ctx, skus)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch prices: %v", err)
	}

	// 3. Calculate totals
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
			Name:       dbItem.Name,
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


// CreateItem handles the gRPC request to add or update an item in the catalog
func (h *PricingHandler) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.CreateItemResponse, error) {
	// 1. Convert the Proto request into a Store Item struct
	item := store.Item{
		Sku:       req.GetSku(),
		Name:      req.GetName(),
		Brand:     req.GetBrand(),
		UnitPrice: req.GetUnitPrice(),
	}

	// 2. Call the "Chef" (Store) to perform the Upsert
	id, err := h.store.UpsertItem(ctx, item)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to upsert item: %v", err)
	}

	// 3. Return the newly created or updated ID
	return &pb.CreateItemResponse{
		Id: int32(id),
	}, nil
}

// GetPrice handles the gRPC request for looking up a single item
func (h *PricingHandler) GetPrice(ctx context.Context, req *pb.GetPriceRequest) (*pb.GetPriceResponse, error) {
	item, err := h.store.GetItem(ctx, req.GetSku())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "item with SKU %s not found", req.GetSku())
	}

	return &pb.GetPriceResponse{
		Id:        int32(item.ID),
		Sku:       item.Sku,
		Name:      item.Name,
		Brand:     item.Brand,
		UnitPrice: item.UnitPrice,
	}, nil
}