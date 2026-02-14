package client

import (
	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	"encoding/json"
	"net/http"
)

type HistoryHandler struct {
	OrderStore *store.OrderStore
}

// ServeHTTP returns historical orders for the authenticated client.
func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	history, _ := h.OrderStore.GetOrdersByClientID(r.Context(), userID)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": history})
}
