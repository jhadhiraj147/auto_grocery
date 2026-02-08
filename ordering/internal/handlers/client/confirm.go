package client

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

type ConfirmOrderHandler struct {
	OrderStore      *store.OrderStore
	InventoryClient pb.InventoryServiceClient
}

func (h *ConfirmOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if order.Status == "COMPLETED" {
		http.Error(w, "Already completed", http.StatusConflict)
		return
	}

	dbItems, _ := h.OrderStore.GetOrderItems(r.Context(), req.OrderID)
	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	grpcResp, err := h.InventoryClient.Checkout(r.Context(), &pb.CheckoutRequest{
		OrderId: req.OrderID, Items: protoItems,
	})
	if err != nil || !grpcResp.Success {
		http.Error(w, "Checkout failed", http.StatusConflict)
		return
	}

	h.OrderStore.UpdateOrderStatus(r.Context(), req.OrderID, "COMPLETED", grpcResp.TotalPrice)
	
	// Return Full Receipt Data
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "completed",
		"order_id":    req.OrderID,
		"total_price": grpcResp.TotalPrice,
		"timestamp":   order.CreatedAt, // <--- ADDED
	})
}