package truck

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/mq" // Import MQ
	"auto_grocery/ordering/internal/store"
)

type WebhookHandler struct {
	RestockStore *store.RestockStore
	Analytics    *mq.AnalyticsPublisher
}

// ServeHTTP processes restock completion webhooks and updates restock order status.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[truck-webhook] request received")
	// Decode webhook payload.
	var req struct {
		OrderID   string  `json:"order_id"`
		Status    string  `json:"status"`
		TotalCost float64 `json:"total_cost"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[truck-webhook] invalid json err=%v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[truck-webhook] payload order_id=%s status=%s total_cost=%.2f", req.OrderID, req.Status, req.TotalCost)

	// Persist restock order status update.
	err := h.RestockStore.UpdateOrderStatus(r.Context(), req.OrderID, req.Status, req.TotalCost)
	if err != nil {
		log.Printf("[truck-webhook] update failed order_id=%s err=%v", req.OrderID, err)
		http.Error(w, "Failed to update order status", http.StatusInternalServerError)
		return
	}
	log.Printf("[truck-webhook] order updated order_id=%s status=%s", req.OrderID, req.Status)

	// Publish analytics asynchronously.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		order, err := h.RestockStore.GetRestockOrder(ctx, req.OrderID)
		if err != nil {
			log.Printf("[truck-webhook] WARN analytics fetch restock details failed order_id=%s err=%v", req.OrderID, err)
			return
		}

		if h.Analytics != nil {
			duration := time.Since(order.CreatedAt).Seconds()
			log.Printf("[truck-webhook] analytics publish order_id=%s status=%s duration=%.2fs", req.OrderID, req.Status, duration)
			h.Analytics.Publish(req.OrderID, req.Status, duration)
		}
	}()

	// Acknowledge webhook delivery.
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Restock finalized successfully",
	})
}
