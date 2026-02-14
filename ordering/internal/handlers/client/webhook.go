package client

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/mq"
	"auto_grocery/ordering/internal/store"
)

type WebhookHandler struct {
	OrderStore *store.OrderStore
	Analytics  *mq.AnalyticsPublisher
}

type WebhookPayload struct {
	OrderID    string  `json:"order_id"`
	Status     string  `json:"status"`
	TotalPrice float64 `json:"total_price"`
}

// ServeHTTP processes inventory completion webhooks and updates client order status.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Decode webhook payload.
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Persist final order status.
	err := h.OrderStore.UpdateOrderStatus(r.Context(), payload.OrderID, payload.Status, payload.TotalPrice)
	if err != nil {
		log.Printf("[client-webhook] ERROR failed to update order order_id=%s err=%v", payload.OrderID, err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}

	// Publish analytics asynchronously.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		order, err := h.OrderStore.GetOrderByID(ctx, payload.OrderID)
		if err != nil {
			log.Printf("[client-webhook] WARN analytics fetch order details failed order_id=%s err=%v", payload.OrderID, err)
			return
		}

		if h.Analytics != nil {
			duration := time.Since(order.CreatedAt).Seconds()
			h.Analytics.Publish(payload.OrderID, payload.Status, duration)
		}
	}()

	// Acknowledge webhook delivery.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received"))
}
