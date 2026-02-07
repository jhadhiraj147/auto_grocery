package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"auto_grocery/ordering/internal/auth" // <--- Import this to access UserKey
	"auto_grocery/ordering/internal/store"
	// IMPORTANT: This points to the proto generated inside the Ordering service
	pb "auto_grocery/ordering/proto"
	
	"github.com/google/uuid"
)

type OrderHandler struct {
	orderStore      *store.OrderStore
	inventoryClient pb.InventoryServiceClient
}

// NewOrderHandler injects the Database Store AND the gRPC Client
func NewOrderHandler(os *store.OrderStore, inv pb.InventoryServiceClient) *OrderHandler {
	return &OrderHandler{
		orderStore:      os,
		inventoryClient: inv,
	}
}

// ---------------------------------------------------------------------
// 1. REVIEW ORDER (Step 1: Reserve Stock & Show User)
// ---------------------------------------------------------------------
// URL: POST /api/client/order/preview
func (h *OrderHandler) PreviewOrder(w http.ResponseWriter, r *http.Request) {
	// --- AUTH CHECK ---
	// We extract the User ID that the Middleware found in the JWT
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized: User ID missing", http.StatusUnauthorized)
		return
	}

	var req struct {
		Items []struct {
			Sku      string `json:"sku"`
			Quantity int32  `json:"quantity"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// A. Generate Order ID (We need this to hold the stock!)
	orderUUID := uuid.New().String()

	// B. Prepare gRPC Request
	protoItems := make(map[string]int32)
	for _, item := range req.Items {
		protoItems[item.Sku] = item.Quantity
	}

	// C. CALL INVENTORY: RESERVE ITEMS
	// This locks the stock so nobody else can take it while user decides.
	grpcReq := &pb.ReserveItemsRequest{
		OrderId: orderUUID,
		Items:   protoItems,
	}

	grpcResp, err := h.inventoryClient.ReserveItems(r.Context(), grpcReq)
	if err != nil {
		http.Error(w, "Failed to connect to inventory service", http.StatusInternalServerError)
		return
	}

	if !grpcResp.Success {
		http.Error(w, "One or more items are out of stock", http.StatusConflict)
		return
	}

	// D. SAVE TO DB AS "PENDING"
	// We MUST save this now so we have a record to Confirm or Cancel later.
	var dbItems []store.GroceryOrderItem
	for sku, qty := range grpcResp.Items {
		dbItems = append(dbItems, store.GroceryOrderItem{
			Sku:      sku,
			Quantity: int(qty),
		})
	}

	orderHeader := store.GroceryOrder{
		OrderID:    orderUUID,
		ClientID:   userID, // <--- Using the REAL ID now!
		Status:     "PENDING",
		TotalPrice: 0.0,
	}

	err = h.orderStore.CreateGroceryOrder(r.Context(), orderHeader, dbItems)
	if err != nil {
		// If we can't save the order, we must release the stock immediately!
		h.inventoryClient.ReleaseItems(r.Context(), &pb.ReleaseItemsRequest{OrderId: orderUUID})
		http.Error(w, "System error: Could not create order record", http.StatusInternalServerError)
		return
	}

	// E. Respond with "Reserved" status
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "reserved",
		"order_id": orderUUID,
		"items":    grpcResp.Items,
		"message":  "Items reserved. Please Confirm or Cancel.",
	})
}

// ---------------------------------------------------------------------
// 2. CONFIRM ORDER (Secure Owner Check)
// ---------------------------------------------------------------------
// URL: POST /api/client/order/confirm
func (h *OrderHandler) ConfirmOrder(w http.ResponseWriter, r *http.Request) {
	// A. Get User ID from Token (The "Who is this?" check)
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// B. Fetch Order Header to check ownership
	order, err := h.orderStore.GetOrderByID(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// C. SECURITY CHECK: Does this order belong to this user?
	if order.ClientID != userID {
		// We return "Not Found" to not leak existence of other orders
		http.Error(w, "Order not found", http.StatusNotFound) 
		return
	}

	// D. Check if already completed (Idempotency)
	if order.Status == "COMPLETED" {
		http.Error(w, "Order already completed", http.StatusConflict)
		return
	}

	// E. Fetch Items (Now we know it's safe)
	dbItems, err := h.orderStore.GetOrderItems(r.Context(), req.OrderID)
	if err != nil || len(dbItems) == 0 {
		http.Error(w, "Order items not found", http.StatusInternalServerError)
		return
	}

	// F. Prepare gRPC Request
	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	// G. Call Inventory Checkout
	grpcReq := &pb.CheckoutRequest{
		OrderId: req.OrderID,
		Items:   protoItems, 
	}

	grpcResp, err := h.inventoryClient.Checkout(r.Context(), grpcReq)
	if err != nil {
		http.Error(w, "Checkout failed", http.StatusInternalServerError)
		return
	}

	if !grpcResp.Success {
		http.Error(w, "Checkout denied by Inventory", http.StatusConflict)
		return
	}

	// H. Update Status
	err = h.orderStore.UpdateOrderStatus(r.Context(), req.OrderID, "COMPLETED", grpcResp.TotalPrice)
	if err != nil {
		fmt.Printf("Error updating status: %v\n", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "completed",
		"order_id":    req.OrderID,
		"total_price": grpcResp.TotalPrice,
	})
}


// ---------------------------------------------------------------------
// 3. CANCEL ORDER (Secure Owner Check)
// ---------------------------------------------------------------------
// URL: POST /api/client/order/cancel
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	// A. Get User ID (Who is calling?)
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// B. SECURITY CHECK: Fetch Order Header & Check Owner
	// We must verify the order belongs to this user BEFORE we delete it.
	order, err := h.orderStore.GetOrderByID(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if order == nil || order.ClientID != userID {
		// If it doesn't exist OR it belongs to someone else -> 404
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// C. Check status (Optional but good)
	if order.Status == "COMPLETED" {
		http.Error(w, "Cannot cancel a completed order", http.StatusConflict)
		return
	}

	// D. FETCH ITEMS FROM DB (To release stock)
	dbItems, err := h.orderStore.GetOrderItems(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Order items not found", http.StatusNotFound)
		return
	}

	// E. PREPARE RELEASE REQUEST
	protoItems := make(map[string]int32)
	for _, item := range dbItems {
		protoItems[item.Sku] = int32(item.Quantity)
	}

	// F. CALL INVENTORY: RELEASE ITEMS
	_, err = h.inventoryClient.ReleaseItems(r.Context(), &pb.ReleaseItemsRequest{
		OrderId: req.OrderID,
		Items:   protoItems, 
	})
	
	if err != nil {
		fmt.Println("Warning: Failed to notify inventory to release items:", err)
	}

	// G. DELETE ORDER FROM DB
	err = h.orderStore.DeleteOrder(r.Context(), req.OrderID)
	if err != nil {
		http.Error(w, "Failed to delete order", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "cancelled",
		"message": "Order deleted and stock released.",
	})
}

// ---------------------------------------------------------------------
// 4. GET ORDER HISTORY
// ---------------------------------------------------------------------
func (h *OrderHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	// A. Get User ID from Token (The "Who is this?" check)
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// B. Query DB using the Token's ID
	// This ensures User 55 only sees rows where client_id = 55
	history, err := h.orderStore.GetOrdersByClientID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   history,
	})
}

// ---------------------------------------------------------------------
// 5. GET RECEIPT (Last Order)
// ---------------------------------------------------------------------
func (h *OrderHandler) GetLastOrder(w http.ResponseWriter, r *http.Request) {
	// A. Get User ID from Token
	userID, ok := r.Context().Value(auth.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// B. Query DB using the Token's ID
	lastOrder, err := h.orderStore.GetLastOrderByClientID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to fetch last order", http.StatusInternalServerError)
		return
	}

	// Handle case where user has no orders yet
	if lastOrder == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   nil,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   lastOrder,
	})
}