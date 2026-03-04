package client

import (
	"auto_grocery/ordering/internal/auth"
	"encoding/json"
	"net/http"
	"strconv"
)

// History returns historical orders for the authenticated client (default last 50, max 200).
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 50
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	history, err := h.orderStore.GetOrdersByClientID(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": history})
}
