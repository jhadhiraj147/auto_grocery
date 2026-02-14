package handler

import (
	"bytes"
	"context"
	"encoding/json"
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
	store             *store.Store
	memoryStore       *store.MemoryStore
	publisher         *mq.Publisher
	pricingClient     pb.PricingServiceClient
	orderWebhookURL   string
	restockWebhookURL string
}

// NewInventoryHandler constructs the inventory gRPC handler and integration clients.
func NewInventoryHandler(
	s *store.Store,
	ms *store.MemoryStore,
	pub *mq.Publisher,
	p pb.PricingServiceClient,
	orderWebhookURL string,
	restockWebhookURL string,
) *InventoryHandler {
	return &InventoryHandler{
		store:             s,
		memoryStore:       ms,
		publisher:         pub,
		pricingClient:     p,
		orderWebhookURL:   orderWebhookURL,
		restockWebhookURL: restockWebhookURL,
	}
}

// CheckAvailability returns current stock details for requested SKUs.
func (h *InventoryHandler) CheckAvailability(ctx context.Context, req *pb.CheckAvailabilityRequest) (*pb.CheckAvailabilityResponse, error) {
	log.Printf("[inventory] check-availability skus=%v", req.GetSkus())
	if len(req.GetSkus()) == 0 {
		return &pb.CheckAvailabilityResponse{Items: map[string]*pb.StockLevel{}}, nil
	}

	dbItems, err := h.store.GetBatchItems(ctx, req.GetSkus())
	if err != nil {
		log.Printf("[inventory] check-availability db error: %v", err)
		return nil, err
	}

	respItems := make(map[string]*pb.StockLevel)
	for _, sku := range req.GetSkus() {
		if item, exists := dbItems[sku]; exists {
			respItems[sku] = &pb.StockLevel{
				Sku:               item.SKU,
				Name:              item.Name,
				AisleType:         item.AisleType,
				QuantityAvailable: int32(item.Quantity),
			}
		}
	}

	return &pb.CheckAvailabilityResponse{Items: respItems}, nil
}

// ReserveItems attempts atomic stock reservation for a client order.
func (h *InventoryHandler) ReserveItems(ctx context.Context, req *pb.ReserveItemsRequest) (*pb.ReserveItemsResponse, error) {
	log.Printf("[inventory] reserve request order=%s items=%v", req.GetOrderId(), req.GetItems())
	if req.GetOrderId() == "" {
		return &pb.ReserveItemsResponse{OrderId: "", Success: false, ErrorMessage: "missing order_id"}, nil
	}
	if len(req.GetItems()) == 0 {
		return &pb.ReserveItemsResponse{OrderId: req.GetOrderId(), Success: false, ErrorMessage: "no items to reserve"}, nil
	}

	reserved, err := h.store.ReserveStock(ctx, req.GetItems())
	if err != nil {
		log.Printf("[inventory] reserve db error order=%s err=%v", req.GetOrderId(), err)
		return nil, err
	}
	log.Printf("[inventory] reserve result order=%s reserved=%v", req.GetOrderId(), reserved)

	allReserved := true
	for sku, wanted := range req.GetItems() {
		if reserved[sku] < wanted {
			allReserved = false
			break
		}
	}

	if !allReserved {
		if len(reserved) > 0 {
			_ = h.store.ReleaseStock(ctx, reserved)
		}
		log.Printf("[inventory] reserve rejected order=%s reason=insufficient_stock", req.GetOrderId())
		return &pb.ReserveItemsResponse{
			OrderId:      req.GetOrderId(),
			Success:      false,
			ErrorMessage: "insufficient stock",
		}, nil
	}
	log.Printf("[inventory] reserve success order=%s", req.GetOrderId())

	return &pb.ReserveItemsResponse{OrderId: req.GetOrderId(), Success: true}, nil
}

// ReleaseItems restores previously reserved stock quantities.
func (h *InventoryHandler) ReleaseItems(ctx context.Context, req *pb.ReleaseItemsRequest) (*pb.ReleaseItemsResponse, error) {
	log.Printf("[inventory] release request order=%s items=%v", req.GetOrderId(), req.GetItems())
	if len(req.GetItems()) == 0 {
		return &pb.ReleaseItemsResponse{Success: true}, nil
	}

	if err := h.store.ReleaseStock(ctx, req.GetItems()); err != nil {
		log.Printf("[inventory] release failed order=%s err=%v", req.GetOrderId(), err)
		return nil, err
	}
	log.Printf("[inventory] release success order=%s", req.GetOrderId())

	return &pb.ReleaseItemsResponse{Success: true}, nil
}

// ProcessCustomerOrder persists cart items and dispatches robots for client orders.
func (h *InventoryHandler) ProcessCustomerOrder(ctx context.Context, req *pb.ProcessCustomerOrderRequest) (*pb.ProcessCustomerOrderResponse, error) {
	orderID := req.GetOrderId()
	log.Printf("[inventory] INFO processing customer order=%s", orderID)
	log.Printf("[inventory] process-customer order=%s items=%v", orderID, req.GetItems())

	// 1. Save to Redis DB 0 (Client)
	if err := h.memoryStore.SaveOrderItems(ctx, orderID, req.GetItems()); err != nil {
		log.Printf("[inventory] failed to save order in redis order=%s err=%v", orderID, err)
		return nil, err
	}

	// 2. Fetch Aisle info from DB
	robotItems := h.prepareRobotItems(ctx, req.GetItems())

	// 3. Dispatch Robots
	h.assignRobots(orderID, "CUSTOMER", robotItems)
	log.Printf("[inventory] process-customer dispatched order=%s", orderID)

	return &pb.ProcessCustomerOrderResponse{
		Success: true,
		Message: "Customer order processed and robots dispatched.",
	}, nil
}

// RestockItemsOrder persists restock payloads and dispatches robots.
func (h *InventoryHandler) RestockItemsOrder(ctx context.Context, req *pb.RestockItemsOrderRequest) (*pb.RestockItemsOrderResponse, error) {
	orderID := req.GetOrderId()
	log.Printf("[inventory] INFO processing restock order=%s", orderID)

	// 1. Save to Redis DB 1 (Restock)
	if err := h.memoryStore.SaveRestockItems(ctx, orderID, req.GetItems()); err != nil {
		return nil, err
	}

	// 2. Map items directly (Aisle info is already in the request for trucks)
	robotItems := make(map[string]mq.ItemDetails)
	for _, item := range req.GetItems() {
		robotItems[item.GetSku()] = mq.ItemDetails{
			Quantity: item.GetQuantity(),
			Aisle:    item.GetAisleType(),
		}
	}

	// 3. Dispatch Robots
	h.assignRobots(orderID, "RESTOCK", robotItems)

	return &pb.RestockItemsOrderResponse{Success: true}, nil
}

// assignRobots broadcasts robot work assignments for an order.
func (h *InventoryHandler) assignRobots(orderID string, orderType string, items map[string]mq.ItemDetails) {
	log.Printf("[inventory] INFO dispatching robots type=%s order=%s", orderType, orderID)
	log.Printf("[inventory] dispatch order=%s type=%s items=%v", orderID, orderType, items)
	if err := h.publisher.SendRobotCommand(orderID, orderType, items); err != nil {
		log.Printf("[inventory] ERROR zmq broadcast failed order=%s err=%v", orderID, err)
	}
}

// ReportJobStatus records robot progress and triggers order finalization once complete.
func (h *InventoryHandler) ReportJobStatus(ctx context.Context, req *pb.ReportJobStatusRequest) (*pb.ReportJobStatusResponse, error) {
	orderID := req.GetOrderId()
	orderType := req.GetOrderType()
	status := req.GetStatus()

	// Explicit check based on the type reported by the robot
	isRestock := (orderType == "RESTOCK")

	var count int64
	var err error

	if isRestock {
		count, err = h.memoryStore.IncrementRestockRobotCount(ctx, orderID)
	} else {
		count, err = h.memoryStore.IncrementClientRobotCount(ctx, orderID)
	}
	if err != nil {
		log.Printf("[inventory] ERROR failed to increment robot count order=%s type=%s err=%v", orderID, orderType, err)
		return nil, err
	}

	log.Printf("[inventory] robot status received order=%s type=%s status=%s count=%d", orderID, orderType, status, count)

	if count >= 5 {
		marked, markErr := h.memoryStore.TryMarkOrderFinalized(ctx, orderID, isRestock)
		if markErr != nil {
			log.Printf("[inventory] ERROR failed to mark order finalized order=%s err=%v", orderID, markErr)
			return nil, markErr
		}
		if marked {
			log.Printf("[inventory] INFO robot threshold reached order=%s finalizing", orderID)
			go h.finalizeOrder(orderID, isRestock)
		}
	}

	return &pb.ReportJobStatusResponse{Success: true}, nil
}

// finalizeOrder routes to client or restock finalization workflow.
func (h *InventoryHandler) finalizeOrder(orderID string, isRestock bool) {
	if isRestock {
		h.finalizeRestock(orderID)
	} else {
		h.finalizeClientOrder(orderID)
	}
}

// finalizeRestock updates stock, pricing metrics, ordering status, and cache cleanup.
func (h *InventoryHandler) finalizeRestock(orderID string) {
	ctx := context.Background()
	items, err := h.memoryStore.GetRestockItems(ctx, orderID)
	if err != nil {
		log.Printf("Finalize Restock Error: Could not find items for %s: %v", orderID, err)
		return
	}

	var totalCost float64
	latestUnitCostBySKU := make(map[string]float64)
	updatedSKUs := make(map[string]struct{})

	for _, pi := range items {
		totalCost += pi.GetUnitCost() * float64(pi.GetQuantity())

		// Convert Proto Timestamp to Go Time
		mfd := pi.GetMfdDate().AsTime()
		expiry := pi.GetExpiryDate().AsTime()

		err := h.store.UpsertStock(ctx, store.StockItem{
			SKU:        pi.GetSku(),
			Name:       pi.GetName(),
			AisleType:  pi.GetAisleType(),
			Quantity:   int(pi.GetQuantity()),
			UnitCost:   pi.GetUnitCost(),
			MfdDate:    mfd,
			ExpiryDate: expiry,
		})
		if err != nil {
			log.Printf("Failed to upsert stock for %s: %v", pi.GetSku(), err)
			continue
		}

		latestUnitCostBySKU[pi.GetSku()] = pi.GetUnitCost()
		updatedSKUs[pi.GetSku()] = struct{}{}
	}

	var updatedSKUList []string
	for sku := range updatedSKUs {
		updatedSKUList = append(updatedSKUList, sku)
	}

	var pricingUpdates []*pb.StockMetric
	if len(updatedSKUList) > 0 {
		currentItems, batchErr := h.store.GetBatchItems(ctx, updatedSKUList)
		if batchErr != nil {
			log.Printf("[inventory] WARN failed to fetch current stock for pricing updates err=%v", batchErr)
		} else {
			for _, sku := range updatedSKUList {
				currentItem, exists := currentItems[sku]
				if !exists {
					log.Printf("[inventory] WARN missing current stock row sku=%s during pricing update", sku)
					continue
				}

				pricingUpdates = append(pricingUpdates, &pb.StockMetric{
					Sku:      sku,
					Quantity: int32(currentItem.Quantity),
					UnitCost: latestUnitCostBySKU[sku],
				})
			}
		}
	}

	go func(metrics []*pb.StockMetric) {
		if len(metrics) == 0 {
			return
		}
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := h.pricingClient.UpdateStockMetrics(bgCtx, &pb.UpdateStockMetricsRequest{Updates: metrics})
		if err != nil {
			log.Printf("[inventory] WARN async pricing metric update failed err=%v", err)
		}
	}(pricingUpdates)

	h.callWebhook(orderID, totalCost, h.restockWebhookURL, "total_cost")
	h.memoryStore.DeleteOrderData(ctx, orderID, true)
}

// finalizeClientOrder computes final bill, updates ordering status, and clears cache state.
func (h *InventoryHandler) finalizeClientOrder(orderID string) {
	log.Printf("[inventory] finalize-client start order=%s", orderID)
	ctx := context.Background()
	items, err := h.memoryStore.GetOrderItems(ctx, orderID)
	if err != nil {
		log.Printf("Finalize Client Order Error: %v", err)
		return
	}
	log.Printf("[inventory] finalize-client redis items order=%s items=%v", orderID, items)

	var cartItems []*pb.CartItem
	for sku, qty := range items {
		cartItems = append(cartItems, &pb.CartItem{Sku: sku, Quantity: qty})
	}

	resp, err := h.pricingClient.CalculateBill(ctx, &pb.CalculateBillRequest{Items: cartItems})
	finalPrice := 0.0
	if err == nil {
		finalPrice = resp.GetGrandTotal()
		log.Printf("[inventory] pricing bill success order=%s total=%.2f", orderID, finalPrice)
	} else {
		log.Printf("[inventory] pricing bill failed order=%s err=%v", orderID, err)
	}

	h.callWebhook(orderID, finalPrice, h.orderWebhookURL, "total_price")
	h.memoryStore.DeleteOrderData(ctx, orderID, false)
	log.Printf("[inventory] finalize-client cleanup complete order=%s", orderID)
}

// callWebhook posts completion updates to ordering internal endpoints.
func (h *InventoryHandler) callWebhook(orderID string, price float64, url string, priceKey string) {
	payload := map[string]interface{}{
		"order_id": orderID,
		"status":   "COMPLETED",
		priceKey:   price,
	}
	jsonBytes, _ := json.Marshal(payload)
	log.Printf("[inventory] webhook sending order=%s url=%s payload=%s", orderID, url, string(jsonBytes))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", os.Getenv("INTERNAL_SECRET"))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Webhook failed for %s: %v", orderID, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[inventory] webhook response order=%s url=%s status=%d", orderID, url, resp.StatusCode)
}

// prepareRobotItems joins sku quantities with aisle metadata for robot routing.
func (h *InventoryHandler) prepareRobotItems(ctx context.Context, items map[string]int32) map[string]mq.ItemDetails {
	skus := make([]string, 0, len(items))
	for sku := range items {
		skus = append(skus, sku)
	}

	dbItems, _ := h.store.GetBatchItems(ctx, skus)
	robotItems := make(map[string]mq.ItemDetails)
	for sku, qty := range items {
		aisle := "Unknown"
		if item, exists := dbItems[sku]; exists {
			aisle = item.AisleType
		} else {
			log.Printf("[inventory] aisle lookup missing sku=%s, defaulting Unknown", sku)
		}
		robotItems[sku] = mq.ItemDetails{Quantity: qty, Aisle: aisle}
	}
	log.Printf("[inventory] prepared robot map items=%v", robotItems)
	return robotItems
}
