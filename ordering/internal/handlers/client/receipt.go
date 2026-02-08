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
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	lastOrder, _ := h.OrderStore.GetLastOrderByClientID(r.Context(), userID)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": lastOrder})
}