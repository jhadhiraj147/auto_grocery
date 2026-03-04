package client

import (
	"encoding/json"
	"log"
	"net/http"

	"auto_grocery/ordering/internal/auth"
	pb "auto_grocery/ordering/proto"
)

// ConfirmOrder confirms a pending client order and triggers robot dispatch through inventory.
func (h *Handler) ConfirmOrder(w http.ResponseWriter, r *http.Request) {
	// Authenticate and validate request payload.
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		log.Printf("[confirm] unauthorized request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var reqBody struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("[confirm] invalid json for user=%d: %v", userID, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if reqBody.OrderID == "" {
		log.Printf("[confirm] missing order_id for user=%d", userID)
		http.Error(w, "Missing order_id", http.StatusBadRequest)
		return
	}
	log.Printf("[confirm] request user=%d order=%s", userID, reqBody.OrderID)

	// Verify order ownership and state before dispatch.
	order, err := h.orderStore.GetOrderByID(r.Context(), reqBody.OrderID)
	if err != nil {
		log.Printf("[confirm] db error order=%s user=%d err=%v", reqBody.OrderID, userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		log.Printf("[confirm] order not found order=%s user=%d", reqBody.OrderID, userID)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if order.ClientID != userID {
		log.Printf("[confirm] forbidden order=%s owner=%d caller=%d", reqBody.OrderID, order.ClientID, userID)
		http.Error(w, "Forbidden: You do not own this order", http.StatusForbidden)
		return
	}
	if order.Status != "PENDING" {
		log.Printf("[confirm] invalid status order=%s status=%s", reqBody.OrderID, order.Status)
		http.Error(w, "Order is not in PENDING state", http.StatusConflict)
		return
	}

	// Fetch trusted order items from storage.
	dbItems, err := h.orderStore.GetOrderItems(r.Context(), reqBody.OrderID)
	if err != nil {
		log.Printf("[confirm] failed to load order items order=%s err=%v", reqBody.OrderID, err)
		http.Error(w, "Failed to retrieve order items", http.StatusInternalServerError)
		return
	}
	if len(dbItems) == 0 {
		// Shouldn't happen outside of data corruption, but prevents dispatching robots with nothing.
		log.Printf("[confirm] order has no items order=%s user=%d", reqBody.OrderID, userID)
		http.Error(w, "Order has no items", http.StatusConflict)
		return
	}

	// Convert stored items to the gRPC request shape.
	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	// Atomically transition PENDING → PROCESSING. If another request (e.g. a concurrent cancel)
	// already changed the status, this returns false and we abort — eliminating the TOCTOU race.
	claimed, err := h.orderStore.TransitionOrderStatus(r.Context(), reqBody.OrderID, userID, "PENDING", "PROCESSING")
	if err != nil {
		log.Printf("[confirm] db error during status transition order=%s err=%v", reqBody.OrderID, err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}
	if !claimed {
		log.Printf("[confirm] status transition failed (race or wrong state) order=%s", reqBody.OrderID)
		http.Error(w, "Order is no longer in PENDING state", http.StatusConflict)
		return
	}
	log.Printf("[confirm] status PROCESSING set atomically order=%s items=%v", reqBody.OrderID, protoItems)

	grpcReq := &pb.ProcessCustomerOrderRequest{
		OrderId: reqBody.OrderID,
		Items:   protoItems,
	}

	// Use the request context so the operation is cancelled if the client disconnects.
	resp, err := h.inventoryClient.ProcessCustomerOrder(r.Context(), grpcReq)
	if err != nil {
		// Roll back order status if dispatch fails.
		h.orderStore.UpdateStatus(r.Context(), reqBody.OrderID, "FAILED_DISPATCH")
		log.Printf("[confirm] dispatch failed order=%s err=%v", reqBody.OrderID, err)
		http.Error(w, "Failed to assign robots: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[confirm] dispatch accepted order=%s details=%s", reqBody.OrderID, resp.GetMessage())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Order confirmed. Robots dispatched.",
		"order_id": reqBody.OrderID,
		"status":   "PROCESSING",
		"details":  resp.Message,
	})
}
