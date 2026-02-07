package client

import (
	"encoding/json"
	"net/http"
	"time" // <--- Added this!

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	store *store.ClientStore
}

func NewAuthHandler(s *store.ClientStore) *AuthHandler {
	return &AuthHandler{store: s}
}

// ---------------------------------------------------------------------
// 1. REGISTER (Sign Up)
// ---------------------------------------------------------------------
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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

	err := h.store.CreateSmartClient(r.Context(), client)
	if err != nil {
		http.Error(w, "Registration failed. ID might exist.", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Registered!"})
}

// ---------------------------------------------------------------------
// 2. LOGIN (Get Token & Save Session)
// ---------------------------------------------------------------------
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// A. Find User
	client, err := h.store.GetSmartClient(r.Context(), req.DeviceID)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// B. Check Password
	err = bcrypt.CompareHashAndPassword([]byte(client.PasswordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// C. Generate Tokens
	accessToken, _ := auth.GenerateAccessToken(client.ID, "CLIENT")
	refreshToken, _ := auth.GenerateRefreshToken(client.ID, "CLIENT")

	// D. SAVE REFRESH TOKEN (The Missing Link!)
	// We must tell the DB: "This user is now logged in with this specific token"
	// We match the 7-day expiry used in jwt.go
	expiry := time.Now().Add(7 * 24 * time.Hour) 

	err = h.store.SetRefreshToken(r.Context(), req.DeviceID, refreshToken, expiry)
	if err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// E. Respond
	json.NewEncoder(w).Encode(map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}