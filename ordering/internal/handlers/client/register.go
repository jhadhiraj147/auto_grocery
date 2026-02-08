package client

import (
	"encoding/json"
	"net/http"

	"auto_grocery/ordering/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type RegisterHandler struct {
	Store *store.ClientStore
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	client := store.SmartClient{
		DeviceID:     req.DeviceID,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hashedPwd),
	}

	err := h.Store.CreateSmartClient(r.Context(), client)
	if err != nil {
		http.Error(w, "Registration failed", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}