package client

import (
	"encoding/json"
	"net/http"
	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
)

type LastOrderHandler struct {
	OrderStore *store.OrderStore
}

func (h *LastOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Get UserID from Context
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Fetch the most recent order
	lastOrder, err := h.OrderStore.GetLastOrderByClientID(r.Context(), userID)
	if err != nil {
		// If no order exists, return a 404 so the frontend knows to stop or show 'No Orders'
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No orders found for this user"})
		return
	}

	// 3. Return the order data
	// The frontend will check lastOrder.Status (e.g., "PROCESSING", "COMPLETED", or "FAILED")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   lastOrder,
	})
}