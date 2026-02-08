package client

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
	"github.com/google/uuid"
)

type PreviewOrderHandler struct {
	OrderStore      *store.OrderStore
	InventoryClient pb.InventoryServiceClient
}

func (h *PreviewOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Items []struct {
			Sku      string `json:"sku"`
			Quantity int32  `json:"quantity"`
		} `json:"items"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	orderUUID := uuid.New().String()
	protoItems := make(map[string]int32)
	for _, item := range req.Items {
		protoItems[item.Sku] = item.Quantity
	}

	grpcResp, err := h.InventoryClient.ReserveItems(r.Context(), &pb.ReserveItemsRequest{
		OrderId: orderUUID, Items: protoItems,
	})
	if err != nil || !grpcResp.Success {
		http.Error(w, "Reservation failed", http.StatusConflict)
		return
	}

	var dbItems []store.GroceryOrderItem
	for sku, qty := range grpcResp.Items {
		dbItems = append(dbItems, store.GroceryOrderItem{Sku: sku, Quantity: int(qty)})
	}
	
	h.OrderStore.CreateGroceryOrder(r.Context(), store.GroceryOrder{
		OrderID: orderUUID, ClientID: userID, Status: "PENDING",
	}, dbItems)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "reserved", "order_id": orderUUID, "items": grpcResp.Items,
	})
}