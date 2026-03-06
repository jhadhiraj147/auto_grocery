package client

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"

	"github.com/google/uuid"
)

// PreviewOrder reserves requested items in inventory and persists a pending order.
func (h *Handler) PreviewOrder(w http.ResponseWriter, r *http.Request) {
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

	if len(req.Items) == 0 {
		http.Error(w, "At least one item is required", http.StatusBadRequest)
		return
	}
	for _, item := range req.Items {
		if strings.TrimSpace(item.Sku) == "" {
			http.Error(w, "All items must have a non-empty sku", http.StatusBadRequest)
			return
		}
		if item.Quantity <= 0 {
			http.Error(w, "All item quantities must be greater than zero", http.StatusBadRequest)
			return
		}
	}

	// Enforce single PENDING order per user — prevents stock being held by zombie orders.
	existing, err := h.orderStore.GetPendingOrderForClient(r.Context(), userID)
	if err != nil {
		log.Printf("[preview] pending order check failed user=%d err=%v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		log.Printf("[preview] rejected: user=%d already has pending order=%s", userID, existing.OrderID)
		http.Error(w, "You already have a pending order: "+existing.OrderID+". Cancel it before creating a new one.", http.StatusConflict)
		return
	}

	orderUUID := uuid.New().String()
	protoItems := make(map[string]int32)
	for _, item := range req.Items {
		protoItems[item.Sku] += item.Quantity
	}
	log.Printf("[preview] user=%d items=%v", userID, protoItems)

	grpcResp, err := h.inventoryClient.ReserveItems(r.Context(), &pb.ReserveItemsRequest{
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

	// Resolve actual reserved quantities: partial fills carry a JSON map in error_message.
	actualItems := protoItems
	if errMsg := grpcResp.GetErrorMessage(); errMsg != "" {
		var partialItems map[string]int32
		if jsonErr := json.Unmarshal([]byte(errMsg), &partialItems); jsonErr == nil {
			actualItems = partialItems
			log.Printf("[preview] partial reserve order=%s user=%d actual=%v", orderUUID, userID, actualItems)
		}
	} else {
		log.Printf("[preview] reserve success (full) order=%s user=%d", orderUUID, userID)
	}

	var dbItems []store.GroceryOrderItem
	// Persist only the actually-reserved items (partial subset if applicable).
	for sku, qty := range actualItems {
		dbItems = append(dbItems, store.GroceryOrderItem{Sku: sku, Quantity: int(qty)})
	}

	err = h.orderStore.CreateGroceryOrder(r.Context(), store.GroceryOrder{
		OrderID: orderUUID, ClientID: userID, Status: "PENDING",
	}, dbItems)

	if err != nil {
		log.Printf("[preview] failed to create order row order=%s user=%d err=%v", orderUUID, userID, err)
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}
	log.Printf("[preview] order persisted order=%s user=%d", orderUUID, userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "reserved",
		"order_id": orderUUID,
		"items":    actualItems,
	})
}
