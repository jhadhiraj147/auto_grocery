package client

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Internal-Secret")
	expectedSecret := os.Getenv("INTERNAL_SECRET")
	if secret != expectedSecret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := h.OrderStore.UpdateStatus(r.Context(), payload.OrderID, payload.Status)
	if err != nil {
		log.Printf("❌ Failed to update order %s: %v", payload.OrderID, err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}

	go func() {
		order, err := h.OrderStore.GetOrderByID(r.Context(), payload.OrderID)
		if err == nil && h.Analytics != nil {
			duration := time.Since(order.CreatedAt).Seconds()

			err := h.Analytics.Publish(payload.OrderID, payload.Status, duration)
			if err != nil {
				log.Printf("⚠️ Analytics Publish Failed: %v", err)
			} else {
				log.Printf("📊 Analytics Sent: Order %s took %.2fs", payload.OrderID, duration)
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received"))
}
