package client

import (
	"encoding/json"
	"net/http"
	"strings"

	"auto_grocery/ordering/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// Register registers a new smart client device account.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
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

	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.Email) == "" || len(req.Password) < 6 {
		http.Error(w, "device_id, email are required and password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	client := store.SmartClient{
		DeviceID:     strings.TrimSpace(req.DeviceID),
		Email:        strings.TrimSpace(req.Email),
		Phone:        req.Phone,
		PasswordHash: string(hashedPwd),
	}

	err := h.clientStore.CreateSmartClient(r.Context(), client)
	if err != nil {
		http.Error(w, "Registration failed: device_id or email already exists", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
