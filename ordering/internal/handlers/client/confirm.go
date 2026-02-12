package client

import (
	"context"
	"encoding/json"
	"net/http"
	
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

type ConfirmOrderHandler struct {
	OrderStore      *store.OrderStore
	InventoryClient pb.InventoryServiceClient
}

// Request payload from the Frontend
type ConfirmRequest struct {
	OrderID  string           `json:"order_id"` // <--- NEW: Use the ID from Preview!
	ClientID int              `json:"client_id"`
	Items    map[string]int32 `json:"items"`    // SKU -> Quantity
}

func (h *ConfirmOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Request
	var reqBody ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if reqBody.OrderID == "" {
		http.Error(w, "Missing order_id", http.StatusBadRequest)
		return
	}

	// 2. Validate & Update the EXISTING Order
	// We do NOT create a new order here. We find the "PENDING" order created by Preview
	// and flip it to "PROCESSING".
	// Note: We assume the items in the DB match reqBody.Items because of the Preview/Reservation flow.
	
	err := h.OrderStore.UpdateStatus(r.Context(), reqBody.OrderID, "PROCESSING")
	if err != nil {
		// If error (e.g. order not found), fail immediately
		http.Error(w, "Order not found or update failed: "+err.Error(), http.StatusNotFound)
		return
	}

	// 3. Dispatch Robots (gRPC)
	// We pass the SAME Order ID so the Inventory service knows which reservation to use.
	grpcReq := &pb.AssignRequest{
		OrderId: reqBody.OrderID,
		Items:   reqBody.Items,
	}

	resp, err := h.InventoryClient.AssignRobots(context.Background(), grpcReq)
	if err != nil {
		// If dispatch fails, you might want to revert status to PENDING or mark FAILED
		h.OrderStore.UpdateStatus(r.Context(), reqBody.OrderID, "FAILED_DISPATCH")
		http.Error(w, "Failed to assign robots: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Respond to User
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Order confirmed. Robots dispatched.",
		"order_id": reqBody.OrderID,
		"status":   "PROCESSING", 
		"details":  resp.Message,
	})
}