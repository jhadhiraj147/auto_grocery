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

// ServeHTTP validates a refresh token and issues a new access token.
func (h *RefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Token", http.StatusUnauthorized)
		return
	}
	tokenString := strings.Split(authHeader, " ")[1]

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid Token", http.StatusUnauthorized)
		return
	}

	if claims.TokenType != "REFRESH" {
		http.Error(w, "Invalid Token Type", http.StatusUnauthorized)
		return
	}

	newAccessToken, _ := auth.GenerateAccessToken(claims.UserID, claims.Role)

	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}
