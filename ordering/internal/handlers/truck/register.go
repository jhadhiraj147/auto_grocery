package truck

import (
	"encoding/json"
	"net/http"
	"auto_grocery/ordering/internal/store"
)

type RegisterHandler struct {
	TruckStore *store.TruckStore
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TruckID     string `json:"truck_id"`
		PlateNumber string `json:"plate_number"`
		DriverName  string `json:"driver_name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	truck := store.SmartTruck{
		TruckID: req.TruckID, PlateNumber: req.PlateNumber, DriverName: req.DriverName,
	}
	
	_, err := h.TruckStore.UpsertSmartTruck(r.Context(), truck)
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}