package truck

import (
	"encoding/json"
	"log"
	"net/http"
)

// RestockStatus returns current restock order status for truck frontend polling.
func (h *Handler) RestockStatus(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		log.Printf("[truck-status] missing order_id")
		http.Error(w, "missing order_id", http.StatusBadRequest)
		return
	}
	log.Printf("[truck-status] request order_id=%s", orderID)

	order, err := h.restockStore.GetRestockOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("[truck-status] db error order_id=%s err=%v", orderID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		log.Printf("[truck-status] order not found order_id=%s", orderID)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	log.Printf("[truck-status] response order_id=%s status=%s total_cost=%.2f", order.OrderID, order.Status, order.TotalCost)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   order,
	})
}
