package truck  

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
	"github.com/google/uuid"
)

type RestockHandler struct {
	TruckStore      *store.TruckStore
	RestockStore    *store.RestockStore
	InventoryClient pb.InventoryServiceClient
}

func (h *RestockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Truck Info (Auto-Register)
		TruckID     string `json:"truck_id"`
		PlateNumber string `json:"plate_number"`
		DriverName  string `json:"driver_name"`
		ContactInfo string `json:"contact_info"`
		Location    string `json:"location"`
		
		// Stock Info
		SupplierID string `json:"supplier_id"`
		Items      []struct {
			Sku        string  `json:"sku"`
			Name       string  `json:"name"`
			AisleType  string  `json:"aisle_type"`
			Quantity   int32   `json:"quantity"`
			MfdDate    string  `json:"mfd_date"`
			ExpiryDate string  `json:"expiry_date"`
			UnitCost   float64 `json:"unit_cost"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 1. AUTO-REGISTER / UPDATE TRUCK
	truck := store.SmartTruck{
		TruckID:     req.TruckID,
		PlateNumber: req.PlateNumber,
		DriverName:  req.DriverName,
		ContactInfo: req.ContactInfo,
		Location:    req.Location,
	}

	dbTruckID, err := h.TruckStore.UpsertSmartTruck(r.Context(), truck)
	if err != nil {
		http.Error(w, "Failed to register/update truck info", http.StatusInternalServerError)
		return
	}

	// 2. Prepare gRPC Request
	restockUUID := uuid.New().String()
	var protoItems []*pb.RestockItem
	for _, item := range req.Items {
		protoItems = append(protoItems, &pb.RestockItem{
			Sku: item.Sku, Name: item.Name, AisleType: item.AisleType, Quantity: item.Quantity,
			MfdDate: item.MfdDate, ExpiryDate: item.ExpiryDate, UnitCost: item.UnitCost,
		})
	}

	// 3. Call Inventory
	grpcResp, err := h.InventoryClient.RestockItems(r.Context(), &pb.RestockItemsRequest{
		SupplierId: req.SupplierID, Items: protoItems,
	})
	if err != nil || !grpcResp.Success {
		http.Error(w, "Inventory rejected restock", http.StatusConflict)
		return
	}

	// 4. Save History (Using the DB ID we got from Upsert)
	var dbItems []store.RestockOrderItem
	for _, item := range req.Items {
		dbItems = append(dbItems, store.RestockOrderItem{Sku: item.Sku, Quantity: int(item.Quantity)})
	}

	h.RestockStore.CreateRestockOrder(r.Context(), store.RestockOrder{
		OrderID: restockUUID, TruckID: dbTruckID, Status: "COMPLETED",
	}, dbItems)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success", "order_id": restockUUID, "message": "Truck updated & Stock accepted",
	})
}