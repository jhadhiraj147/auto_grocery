package client

import (
	"encoding/json"
	"net/http"
	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
)

type HistoryHandler struct {
	OrderStore *store.OrderStore
}

func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	history, _ := h.OrderStore.GetOrdersByClientID(r.Context(), userID)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": history})
}