package handler

import (
    "context"
    "fmt"
    "time"

    "auto_grocery/inventory/internal/store"
    pb "auto_grocery/inventory/proto" 
)

type InventoryHandler struct {
    pb.UnimplementedInventoryServiceServer
    store         *store.Store
    pricingClient pb.PricingServiceClient 
}


func NewInventoryHandler(s *store.Store, p pb.PricingServiceClient) *InventoryHandler {
    return &InventoryHandler{
        store:         s,
        pricingClient: p,
    }
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

func (h *InventoryHandler) ReportJobStatus(ctx context.Context, req *pb.ReportJobStatusRequest) (*pb.ReportJobStatusResponse, error) {
    fmt.Printf("🤖 Robot Report: Order=%s Robot=%s Status=%s\n", 
        req.GetOrderId(), req.GetRobotId(), req.GetStatus())
    return &pb.ReportJobStatusResponse{Success: true}, nil
}


func (h *InventoryHandler) BillAndPay(ctx context.Context, req *pb.BillRequest) (*pb.BillResponse, error) {
    fmt.Printf("🛒 Processing Bill & Pay for Order %s...\n", req.GetOrderId())

    var pricingItems []*pb.CartItem
    for sku, qty := range req.GetItems() {
        pricingItems = append(pricingItems, &pb.CartItem{
            Sku:      sku,
            Quantity: qty,
        })
    }

    priceReq := &pb.CalculateBillRequest{Items: pricingItems}
    priceResp, err := h.pricingClient.CalculateBill(ctx, priceReq)
    
    var finalPrice float64 = 0.0
    if err != nil {
        fmt.Printf("⚠️ Pricing Service failed: %v\n", err)
    } else {
        finalPrice = priceResp.GetGrandTotal()
    }
    return &pb.BillResponse{
        OrderId:     req.GetOrderId(),
        Items:       req.GetItems(), 
        Success:     true,
        TotalPrice:  finalPrice,
    }, nil
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