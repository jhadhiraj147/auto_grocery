package client

import (
	"encoding/json"
	"log"
	"net/http"
	"auto_grocery/ordering/internal/store"
)

// WebhookPayload defines the message Inventory sends us
type WebhookPayload struct {
	OrderID    string  `json:"order_id"`
	Status     string  `json:"status"`
	TotalPrice float64 `json:"total_price"`
}

// WebhookHandler struct dependencies
type WebhookHandler struct {
	OrderStore *store.OrderStore
}

// ServeHTTP makes this struct a valid http.Handler
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload WebhookPayload

	// 1. Decode JSON
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}

	log.Printf("🔔 [Webhook] Received update for Order %s -> %s", payload.OrderID, payload.Status)

	// 2. Update the Database Status
	// (Ensure UpdateStatus exists in your store/grocery_orders.go)
	err := h.OrderStore.UpdateStatus(r.Context(), payload.OrderID, payload.Status)
	if err != nil {
		log.Printf("Failed to update DB: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 3. Respond Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Order updated successfully"})
}