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
// 1. REGISTER TRUCK (Logic remains the same)
// ---------------------------------------------------------------------
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

    err := h.truckStore.CreateSmartTruck(r.Context(), truck)
    if err != nil {
        http.Error(w, "Truck ID already exists", http.StatusConflict)
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Truck added to fleet"})
}

// ---------------------------------------------------------------------
// 2. RESTOCK (Updated to include unit_cost and MFD)
// ---------------------------------------------------------------------
func (h *TruckHandler) Restock(w http.ResponseWriter, r *http.Request) {
    var req struct {
        TruckID    string `json:"truck_id"` 
        SupplierID string `json:"supplier_id"`
        Items []struct {
            Sku        string  `json:"sku"`
            Name       string  `json:"name"`
            Quantity   int32   `json:"quantity"`
            MfdDate    string  `json:"mfd_date"`    
            ExpiryDate string  `json:"expiry_date"`
            UnitCost   float64 `json:"unit_cost"`  // Captured from truck delivery
        } `json:"items"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // A. VALIDATE TRUCK
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
            MfdDate:    item.MfdDate,    
            ExpiryDate: item.ExpiryDate,
            UnitCost:   item.UnitCost,   // Passing the cost price to Inventory Service
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

    // D. SAVE HISTORY (Ordering DB logs)
    var dbItems []store.RestockOrderItem
    for _, item := range req.Items {
        dbItems = append(dbItems, store.RestockOrderItem{
            Sku:      item.Sku,
            Quantity: int(item.Quantity),
        })
    }

    orderHeader := store.RestockOrder{
        OrderID: restockUUID,
        TruckID: truck.ID, 
        Status:  "COMPLETED",
    }

    _ = h.restockStore.CreateRestockOrder(r.Context(), orderHeader, dbItems)

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":   "success",
        "order_id": restockUUID,
        "message":  "Inventory updated with cost data",
    })
}