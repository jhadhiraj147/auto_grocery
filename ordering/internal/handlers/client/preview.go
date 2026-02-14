package client

import (
	"encoding/json"
	"log"
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

// ServeHTTP reserves requested items in inventory and persists a pending order.
func (h *PreviewOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		log.Printf("[preview] unauthorized request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Items []struct {
			Sku      string `json:"sku"`
			Quantity int32  `json:"quantity"`
		} `json:"items"`
	}
	// Validate JSON payload.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[preview] invalid json for user=%d: %v", userID, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	orderUUID := uuid.New().String()
	protoItems := make(map[string]int32)
	for _, item := range req.Items {
		protoItems[item.Sku] = item.Quantity
	}
	log.Printf("[preview] user=%d items=%v", userID, protoItems)

	grpcResp, err := h.InventoryClient.ReserveItems(r.Context(), &pb.ReserveItemsRequest{
		OrderId: orderUUID, Items: protoItems,
	})

	// Validate transport and business response success.
	if err != nil || !grpcResp.GetSuccess() {
		if err != nil {
			log.Printf("[preview] reserve grpc failed order=%s user=%d err=%v", orderUUID, userID, err)
		} else {
			log.Printf("[preview] reserve rejected order=%s user=%d reason=%s", orderUUID, userID, grpcResp.GetErrorMessage())
		}
		http.Error(w, "Reservation failed", http.StatusConflict)
		return
	}
	log.Printf("[preview] reserve success order=%s user=%d", orderUUID, userID)

	var dbItems []store.GroceryOrderItem
	// Persist the trusted requested reservation items.
	for sku, qty := range protoItems {
		dbItems = append(dbItems, store.GroceryOrderItem{Sku: sku, Quantity: int(qty)})
	}

	err = h.OrderStore.CreateGroceryOrder(r.Context(), store.GroceryOrder{
		OrderID: orderUUID, ClientID: userID, Status: "PENDING",
	}, dbItems)

	if err != nil {
		log.Printf("[preview] failed to create order row order=%s user=%d err=%v", orderUUID, userID, err)
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}
	log.Printf("[preview] order persisted order=%s user=%d", orderUUID, userID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "reserved",
		"order_id": orderUUID,
		"items":    protoItems,
	})
}
