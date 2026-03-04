package client

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auto_grocery/ordering/internal/auth"

	"golang.org/x/crypto/bcrypt"
)

// Login authenticates a smart client device and returns access/refresh JWT tokens.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.DeviceID == "" || req.Password == "" {
		http.Error(w, "device_id and password are required", http.StatusBadRequest)
		return
	}

	client, err := h.clientStore.GetSmartClient(r.Context(), req.DeviceID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(client.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	accessToken, err := auth.GenerateAccessToken(client.ID, "CLIENT")
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := auth.GenerateRefreshToken(client.ID, "CLIENT")
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	expiry := time.Now().Add(7 * 24 * time.Hour)
	if err = h.clientStore.SetRefreshToken(r.Context(), req.DeviceID, refreshToken, expiry); err != nil {
		// Non-fatal: token still works, but server-side refresh validation won't match.
		// Log the error but still return the tokens so the client isn't blocked.
		log.Printf("[login] WARN failed to persist refresh token device=%s err=%v", req.DeviceID, err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
