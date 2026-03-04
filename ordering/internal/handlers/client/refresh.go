package client

import (
	"encoding/json"
	"net/http"
	"strings"

	"auto_grocery/ordering/internal/auth"
)

// Refresh validates a refresh token and issues a new access token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Token", http.StatusUnauthorized)
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}
	tokenString := parts[1]

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid Token", http.StatusUnauthorized)
		return
	}

	if claims.TokenType != "REFRESH" {
		http.Error(w, "Invalid Token Type", http.StatusUnauthorized)
		return
	}

	newAccessToken, err := auth.GenerateAccessToken(claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}
