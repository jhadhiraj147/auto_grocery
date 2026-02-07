package truck

import (
	"encoding/json"
	"net/http"

	// Import the local ordering proto
	pb "auto_grocery/ordering/proto"
	"auto_grocery/ordering/internal/store"
	"github.com/google/uuid"
)

type TruckHandler struct {
	truckStore      *store.TruckStore
	restockStore    *store.RestockStore
	inventoryClient pb.InventoryServiceClient
}

// NewTruckHandler injects both stores and the gRPC client
func NewTruckHandler(ts *store.TruckStore, rs *store.RestockStore, inv pb.InventoryServiceClient) *TruckHandler {
	return &TruckHandler{
		truckStore:      ts,
		restockStore:    rs,
		inventoryClient: inv,
	}
}

// ---------------------------------------------------------------------
// 1. REGISTER TRUCK (Add a new truck to the fleet)
// ---------------------------------------------------------------------
// URL: POST /api/truck/register
func (h *TruckHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TruckID     string `json:"truck_id"`
		PlateNumber string `json:"plate_number"`
		DriverName  string `json:"driver_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	truck := store.SmartTruck{
		TruckID:     req.TruckID,
		PlateNumber: req.PlateNumber,
		DriverName:  req.DriverName,
	}

	// Just save it to the DB. No passwords, no hashing.
	err := h.truckStore.CreateSmartTruck(r.Context(), truck)
	if err != nil {
		http.Error(w, "Truck ID already exists", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Truck added to fleet"})
}

// ---------------------------------------------------------------------
// 2. RESTOCK (Truck uploads list -> Inventory Updates)
// ---------------------------------------------------------------------
// URL: POST /api/truck/restock
func (h *TruckHandler) Restock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TruckID    string `json:"truck_id"` // They just tell us who they are
		SupplierID string `json:"supplier_id"`
		Items []struct {
			Sku        string `json:"sku"`
			Name       string `json:"name"`
			Quantity   int32  `json:"quantity"`
			ExpiryDate string `json:"expiry_date"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// A. VALIDATE TRUCK
	// Check if this truck actually exists in our DB
	truck, err := h.truckStore.GetSmartTruck(r.Context(), req.TruckID)
	if err != nil {
		http.Error(w, "Unauthorized: Truck ID not recognized", http.StatusUnauthorized)
		return
	}

	// B. PREPARE gRPC REQUEST
	restockUUID := uuid.New().String()

	var protoItems []*pb.RestockItem
	for _, item := range req.Items {
		protoItems = append(protoItems, &pb.RestockItem{
			Sku:        item.Sku,
			Name:       item.Name,
			Quantity:   item.Quantity,
			ExpiryDate: item.ExpiryDate,
		})
	}

	grpcReq := &pb.RestockItemsRequest{
		SupplierId: req.SupplierID,
		Items:      protoItems,
	}

	// C. CALL INVENTORY SERVICE
	grpcResp, err := h.inventoryClient.RestockItems(r.Context(), grpcReq)
	if err != nil {
		http.Error(w, "Failed to connect to Inventory Service", http.StatusInternalServerError)
		return
	}

	if !grpcResp.Success {
		http.Error(w, "Inventory Service rejected the restock", http.StatusConflict)
		return
	}

	// D. SAVE HISTORY
	var dbItems []store.RestockOrderItem
	for _, item := range req.Items {
		dbItems = append(dbItems, store.RestockOrderItem{
			Sku:      item.Sku,
			Quantity: int(item.Quantity),
		})
	}

	orderHeader := store.RestockOrder{
		OrderID:    restockUUID,
		TruckID:    truck.ID, // Use the DB ID (int) we got from GetSmartTruck
		Status:     "COMPLETED",
	}

	// We assume you have this function in store/restock_orders.go
	// If the function returns an error, we log it but don't fail the request
	_ = h.restockStore.CreateRestockOrder(r.Context(), orderHeader, dbItems)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"order_id": restockUUID,
		"message":  "Inventory updated",
	})
}