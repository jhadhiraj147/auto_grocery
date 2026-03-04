package truck

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Webhook processes restock completion webhooks and updates restock order status.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
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
	if req.OrderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}
	// Status machine: only COMPLETED and FAILED are valid terminal states from inventory.
	if req.Status != "COMPLETED" && req.Status != "FAILED" {
		log.Printf("[truck-webhook] rejected invalid status order_id=%s status=%s", req.OrderID, req.Status)
		http.Error(w, "Invalid status: must be COMPLETED or FAILED", http.StatusBadRequest)
		return
	}
	if req.TotalCost < 0 {
		http.Error(w, "total_cost must not be negative", http.StatusBadRequest)
		return
	}
	log.Printf("[truck-webhook] payload order_id=%s status=%s total_cost=%.2f", req.OrderID, req.Status, req.TotalCost)

	// Persist restock order status update.
	err := h.restockStore.UpdateOrderStatus(r.Context(), req.OrderID, req.Status, req.TotalCost)
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

		order, err := h.restockStore.GetRestockOrder(ctx, req.OrderID)
		if err != nil {
			log.Printf("[truck-webhook] WARN analytics fetch restock details failed order_id=%s err=%v", req.OrderID, err)
			return
		}

		if h.analytics != nil {
			duration := time.Since(order.CreatedAt).Seconds()
			log.Printf("[truck-webhook] analytics publish order_id=%s status=%s duration=%.2fs", req.OrderID, req.Status, duration)
			h.analytics.Publish(req.OrderID, req.Status, duration)
		}
	}()

	// Acknowledge webhook delivery.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Restock finalized successfully",
	})
}
