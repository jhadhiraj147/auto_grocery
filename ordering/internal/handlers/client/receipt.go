package client

import (
	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	"encoding/json"
	"log"
	"net/http"
)

type LastOrderHandler struct {
	OrderStore *store.OrderStore
}

// ServeHTTP returns the most recent order for polling current fulfillment state.
func (h *LastOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Resolve authenticated user id from request context.
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		log.Printf("[last-order] unauthorized request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch most recent order for this user.
	lastOrder, err := h.OrderStore.GetLastOrderByClientID(r.Context(), userID)
	if err != nil {
		log.Printf("[last-order] no order for user=%d err=%v", userID, err)
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
