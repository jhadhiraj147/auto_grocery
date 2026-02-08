package client

import (
	"encoding/json"
	"net/http"
	"strings"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/store"
)

type RefreshHandler struct {
	Store *store.ClientStore
}

func (h *RefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Token", http.StatusUnauthorized)
		return
	}
	tokenString := strings.Split(authHeader, " ")[1]

	// 1. Validate Signature
	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid Token", http.StatusUnauthorized)
		return
	}

	// 2. Validate Type (MUST be Refresh)
	if claims.TokenType != "REFRESH" {
		http.Error(w, "Invalid Token Type", http.StatusUnauthorized)
		return
	}

	// 3. TODO: Check DB for revocation (omitted for brevity, but recommended)

	// 4. Issue New Access Token
	newAccessToken, _ := auth.GenerateAccessToken(claims.UserID, claims.Role)

	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}