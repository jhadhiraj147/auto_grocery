package client

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

type CancelOrderHandler struct {
	OrderStore      *store.OrderStore
	InventoryClient pb.InventoryServiceClient
}

// ServeHTTP cancels an order, releases reserved stock, and removes order records.
func (h *CancelOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	order, _ := h.OrderStore.GetOrderByID(r.Context(), req.OrderID)
	if order == nil || order.ClientID != userID {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	dbItems, _ := h.OrderStore.GetOrderItems(r.Context(), req.OrderID)
	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	h.InventoryClient.ReleaseItems(r.Context(), &pb.ReleaseItemsRequest{
		OrderId: req.OrderID, Items: protoItems,
	})

	h.OrderStore.DeleteOrder(r.Context(), req.OrderID)
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}
