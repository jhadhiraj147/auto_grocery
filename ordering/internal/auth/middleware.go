package auth

import (
	"context"
	"net/http"
	"strings"
)

type ContextKey string

const (
	UserKey ContextKey = "userID"
	RoleKey ContextKey = "role"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization Header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Header Format", http.StatusUnauthorized)
			return
		}

		// 2. Check Signature
		claims, err := ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or Expired Token", http.StatusUnauthorized)
			return
		}

		// 3. CHECK TYPE (The new security rule)
		if claims.TokenType != "ACCESS" {
			http.Error(w, "Invalid Token Type: Access Token required", http.StatusUnauthorized)
			return
		}

		// 4. Pass Context
		ctx := context.WithValue(r.Context(), UserKey, claims.UserID)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}