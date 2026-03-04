package client

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Webhook processes inventory completion webhooks and updates client order status.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	// Decode webhook payload.
	var payload struct {
		OrderID    string  `json:"order_id"`
		Status     string  `json:"status"`
		TotalPrice float64 `json:"total_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields.
	if payload.OrderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}
	// Status machine: only COMPLETED and FAILED are valid terminal states from inventory.
	if payload.Status != "COMPLETED" && payload.Status != "FAILED" {
		log.Printf("[client-webhook] rejected invalid status order_id=%s status=%s", payload.OrderID, payload.Status)
		http.Error(w, "Invalid status: must be COMPLETED or FAILED", http.StatusBadRequest)
		return
	}
	if payload.TotalPrice < 0 {
		http.Error(w, "total_price must not be negative", http.StatusBadRequest)
		return
	}

	// Persist final order status.
	err := h.orderStore.UpdateOrderStatus(r.Context(), payload.OrderID, payload.Status, payload.TotalPrice)
	if err != nil {
		log.Printf("[client-webhook] ERROR failed to update order order_id=%s err=%v", payload.OrderID, err)
		http.Error(w, "Database update failed", http.StatusInternalServerError)
		return
	}

	// Publish analytics asynchronously.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		order, err := h.orderStore.GetOrderByID(ctx, payload.OrderID)
		if err != nil {
			log.Printf("[client-webhook] WARN analytics fetch order details failed order_id=%s err=%v", payload.OrderID, err)
			return
		}

		if h.analytics != nil {
			duration := time.Since(order.CreatedAt).Seconds()
			h.analytics.Publish(payload.OrderID, payload.Status, duration)
		}
	}()

	// Acknowledge webhook delivery.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received"))
}
