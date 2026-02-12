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

type ConfirmRequest struct {
	OrderID  string           `json:"order_id"`
	ClientID int              `json:"client_id"`
	Items    map[string]int32 `json:"items"`
}

func (h *ConfirmOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var reqBody ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if reqBody.OrderID == "" {
		http.Error(w, "Missing order_id", http.StatusBadRequest)
		return
	}

	err := h.OrderStore.UpdateStatus(r.Context(), reqBody.OrderID, "PROCESSING")
	if err != nil {
		http.Error(w, "Order not found or update failed: "+err.Error(), http.StatusNotFound)
		return
	}

	grpcReq := &pb.AssignRequest{
		OrderId: reqBody.OrderID,
		Items:   reqBody.Items,
	}

	resp, err := h.InventoryClient.AssignRobots(context.Background(), grpcReq)
	if err != nil {
		h.OrderStore.UpdateStatus(r.Context(), reqBody.OrderID, "FAILED_DISPATCH")
		http.Error(w, "Failed to assign robots: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Order confirmed. Robots dispatched.",
		"order_id": reqBody.OrderID,
		"status":   "PROCESSING",
		"details":  resp.Message,
	})
}
