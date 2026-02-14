package truck

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RestockHandler struct {
	RestockStore    *store.RestockStore
	InventoryClient pb.InventoryServiceClient
}

// ServeHTTP accepts truck manifest payload, stores restock order, and dispatches robots.
func (h *RestockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[truck-restock] request received")
	// 1. Decode Frontend Payload
	var req struct {
		SupplierID   string `json:"supplier_id"`
		SupplierName string `json:"supplier_name"`
		Items        []struct {
			Sku        string  `json:"sku"`
			Name       string  `json:"name"`
			AisleType  string  `json:"aisle_type"`
			Quantity   int32   `json:"quantity"`
			MfdDate    string  `json:"mfd_date"`    // e.g. "2023-10-01T00:00:00Z"
			ExpiryDate string  `json:"expiry_date"` // e.g. "2024-10-01T00:00:00Z"
			UnitCost   float64 `json:"unit_cost"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[truck-restock] invalid json err=%v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[truck-restock] supplier_id=%s supplier_name=%s items=%d", req.SupplierID, req.SupplierName, len(req.Items))

	// 2. Upsert Supplier in DB
	internalSupplierID, err := h.RestockStore.GetSupplierInternalID(r.Context(), req.SupplierID, req.SupplierName)
	if err != nil {
		log.Printf("[truck-restock] failed to resolve supplier supplier_id=%s err=%v", req.SupplierID, err)
		http.Error(w, "Failed to resolve supplier", http.StatusInternalServerError)
		return
	}
	log.Printf("[truck-restock] supplier resolved supplier_id=%s internal_id=%d", req.SupplierID, internalSupplierID)

	// 3. Prepare Order & DB Items
	restockUUID := uuid.New().String()
	var dbItems []store.RestockOrderItem
	var protoItems []*pb.RestockItem
	var estimatedCost float64

	for _, item := range req.Items {
		log.Printf("[truck-restock] item sku=%s aisle=%s qty=%d unit_cost=%.2f", item.Sku, item.AisleType, item.Quantity, item.UnitCost)
		estimatedCost += item.UnitCost * float64(item.Quantity)

		// Parse times for DB and Proto
		mfdTime, _ := time.Parse(time.RFC3339, item.MfdDate)
		expTime, _ := time.Parse(time.RFC3339, item.ExpiryDate)

		// For Ordering Database (Local Store)
		dbItems = append(dbItems, store.RestockOrderItem{
			Sku:        item.Sku,
			Name:       item.Name,
			AisleType:  item.AisleType,
			Quantity:   int(item.Quantity),
			MfdDate:    item.MfdDate,
			ExpiryDate: item.ExpiryDate,
			UnitCost:   item.UnitCost,
		})

		// For Inventory gRPC (Proto)
		protoItems = append(protoItems, &pb.RestockItem{
			Sku:        item.Sku,
			Name:       item.Name,
			AisleType:  item.AisleType,
			Quantity:   item.Quantity,
			MfdDate:    timestamppb.New(mfdTime),
			ExpiryDate: timestamppb.New(expTime),
			UnitCost:   item.UnitCost,
		})
	}

	// 4. Save to Ordering DB as PROCESSING
	order := &store.RestockOrder{
		OrderID:    restockUUID,
		SupplierID: internalSupplierID,
		Status:     "PROCESSING",
		TotalCost:  estimatedCost,
	}

	if err := h.RestockStore.CreateRestockOrder(r.Context(), order, dbItems); err != nil {
		log.Printf("[truck-restock] failed to persist restock order order_id=%s err=%v", restockUUID, err)
		http.Error(w, "Failed to create restock order", http.StatusInternalServerError)
		return
	}
	log.Printf("[truck-restock] order persisted order_id=%s estimated_cost=%.2f", restockUUID, estimatedCost)

	// 5. Trigger Inventory Service (CHANGING TO RestockItemsOrder)
	grpcResp, err := h.InventoryClient.RestockItemsOrder(r.Context(), &pb.RestockItemsOrderRequest{
		OrderId: restockUUID,
		Items:   protoItems,
	})

	if err != nil || !grpcResp.Success {
		if err != nil {
			log.Printf("[truck-restock] inventory dispatch failed order_id=%s err=%v", restockUUID, err)
		} else {
			log.Printf("[truck-restock] inventory rejected dispatch order_id=%s", restockUUID)
		}
		http.Error(w, "Inventory service failed to assign robots", http.StatusInternalServerError)
		return
	}
	log.Printf("[truck-restock] inventory dispatch accepted order_id=%s", restockUUID)

	// 6. Respond immediately to Frontend
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"order_id": restockUUID,
		"message":  "Truck registered. Robots are currently offloading.",
	})
}
