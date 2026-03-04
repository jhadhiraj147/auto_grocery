package client

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/auth"
	pb "auto_grocery/ordering/proto"
)

// CancelOrder cancels an order, releases reserved stock, and removes order records.
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OrderID == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Read the order to distinguish 404 (not found), 403 (wrong owner), and 409 (wrong status).
	order, err := h.orderStore.GetOrderByID(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if order.ClientID != userID {
		http.Error(w, "Order not found", http.StatusNotFound) // don't reveal existence to wrong user
		return
	}
	if order.Status != "PENDING" {
		http.Error(w, "Cannot cancel order in status: "+order.Status, http.StatusConflict)
		return
	}

	// Fetch items BEFORE attempting the atomic claim — needed for stock release.
	dbItems, err := h.orderStore.GetOrderItems(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Failed to retrieve order items", http.StatusInternalServerError)
		return
	}

	// Atomically transition PENDING → CANCELLING. If confirm won the race and already set
	// PROCESSING, this returns false and we abort — stock is NOT released (robots are running).
	claimed, err := h.orderStore.TransitionOrderStatus(r.Context(), req.OrderID, userID, "PENDING", "CANCELLING")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !claimed {
		// Confirm won the race — order is already being processed by robots.
		http.Error(w, "Order has already been dispatched and cannot be cancelled", http.StatusConflict)
		return
	}

	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	_, releaseErr := h.inventoryClient.ReleaseItems(r.Context(), &pb.ReleaseItemsRequest{
		OrderId: req.OrderID, Items: protoItems,
	})
	if releaseErr != nil {
		// Release failed — roll the status back to PENDING so the user can retry.
		h.orderStore.UpdateStatus(r.Context(), req.OrderID, "PENDING")
		http.Error(w, "Failed to release stock reservation", http.StatusInternalServerError)
		return
	}

	if err := h.orderStore.DeleteOrder(r.Context(), req.OrderID); err != nil {
		http.Error(w, "Failed to delete order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}
