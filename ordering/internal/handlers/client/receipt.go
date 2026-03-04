package client

import (
	"auto_grocery/ordering/internal/auth"
	"encoding/json"
	"log"
	"net/http"
)

// LastOrder returns the most recent order for polling current fulfillment state.
func (h *Handler) LastOrder(w http.ResponseWriter, r *http.Request) {
	// Resolve authenticated user id from request context.
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		log.Printf("[last-order] unauthorized request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch most recent order for this user.
	lastOrder, err := h.orderStore.GetLastOrderByClientID(r.Context(), userID)
	if err != nil {
		log.Printf("[last-order] store error for user=%d err=%v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if lastOrder == nil {
		log.Printf("[last-order] no orders for user=%d", userID)
		// Return 404 when no recent order exists for polling clients.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No orders found for this user"})
		return
	}
	log.Printf("[last-order] user=%d order=%s status=%s", userID, lastOrder.OrderID, lastOrder.Status)

	// Return order payload for frontend polling state updates.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   lastOrder,
	})
}
