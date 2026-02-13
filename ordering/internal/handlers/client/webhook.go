package client

import (
	"context"
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
	// 1. Security Check
	secret := r.Header.Get("X-Internal-Secret")
	expectedSecret := os.Getenv("INTERNAL_SECRET")
	if secret != expectedSecret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Decode Payload
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 3. Update Database (Synchronous)
	err := h.OrderStore.UpdateOrderStatus(r.Context(), payload.OrderID, payload.Status, payload.TotalPrice)
	if err != nil {
		log.Printf("❌ Failed to update order %s: %v", payload.OrderID, err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}

	// 4. Fire Analytics (Asynchronous)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		order, err := h.OrderStore.GetOrderByID(ctx, payload.OrderID)
		if err != nil {
			log.Printf("⚠️ Analytics: Failed to fetch order details for %s: %v", payload.OrderID, err)
			return
		}

		if h.Analytics != nil {
			// 🔧 CLEANED UP LOGIC:
			// Because GetOrderByID now returns CreatedAt as time.Local,
			// we can simply use time.Since() which also uses Local time.
			duration := time.Since(order.CreatedAt).Seconds()

			err := h.Analytics.Publish(payload.OrderID, payload.Status, duration)
			if err != nil {
				log.Printf("⚠️ Analytics Publish Failed: %v", err)
			} else {
				log.Printf("📊 Analytics Sent: Order %s took %.2fs", payload.OrderID, duration)
			}
		}
	}()

	// 5. Respond to Inventory Service
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received"))
}