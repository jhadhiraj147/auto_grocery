package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

type ConfirmOrderHandler struct {
	OrderStore      *store.OrderStore
	InventoryClient pb.InventoryServiceClient
}

// Request payload from the Frontend
type ConfirmRequest struct {
	ClientID int              `json:"client_id"`
	Items    map[string]int32 `json:"items"` // SKU -> Quantity
}

func (h *ConfirmOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Request
	var reqBody ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 2. Create Order ID
	// In a real app, you might save to DB first to get an ID. 
	// Here we generate one based on time.
	orderID := "ORD-" + time.Now().Format("150405")

	// 3. Save "PENDING" Order to Database
	// We create a dummy order structure for now
	order := store.GroceryOrder{
		OrderID:  orderID,
		ClientID: reqBody.ClientID,
		Status:   "PENDING",
	}
	
	// Convert map to slice for the store method
	var orderItems []store.GroceryOrderItem
	for sku, qty := range reqBody.Items {
		orderItems = append(orderItems, store.GroceryOrderItem{
			Sku:      sku,
			Quantity: int(qty),
		})
	}

	if err := h.OrderStore.CreateGroceryOrder(r.Context(), order, orderItems); err != nil {
		http.Error(w, "Failed to create order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	grpcReq := &pb.AssignRequest{
		OrderId: orderID,
		Items:   reqBody.Items,
	}

	// This is now an ASYNC trigger. We don't get the price back yet.
	resp, err := h.InventoryClient.AssignRobots(context.Background(), grpcReq)
	if err != nil {
		http.Error(w, "Failed to assign robots: "+err.Error(), http.StatusInternalServerError)
		// Optional: Mark order as FAILED in DB here
		return
	}

	// 5. Respond to User
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Order placed successfully! Robots are working.",
		"order_id": orderID,
		"status":   "PROCESSING", 
		"details":  resp.Message,
	})
}