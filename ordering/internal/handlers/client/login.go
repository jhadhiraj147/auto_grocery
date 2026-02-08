package client

import (
	"encoding/json"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type LoginHandler struct {
	Store *store.ClientStore
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	client, err := h.Store.GetSmartClient(r.Context(), req.DeviceID)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(client.PasswordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	accessToken, _ := auth.GenerateAccessToken(client.ID, "CLIENT")
	refreshToken, _ := auth.GenerateRefreshToken(client.ID, "CLIENT")

	// Save Refresh Token to DB
	expiry := time.Now().Add(7 * 24 * time.Hour)
	h.Store.SetRefreshToken(r.Context(), req.DeviceID, refreshToken, expiry)

	json.NewEncoder(w).Encode(map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}