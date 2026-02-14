package auth

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
)

type ContextKey string

const (
	UserKey ContextKey = "userID"
	RoleKey ContextKey = "role"
)

// AuthMiddleware validates access tokens and injects user context for protected endpoints.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

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

		claims, err := ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or Expired Token", http.StatusUnauthorized)
			return
		}

		if claims.TokenType != "ACCESS" {
			http.Error(w, "Invalid Token Type: Access Token required", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, claims.UserID)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// InternalMiddleware validates internal service-to-service secret headers.
func InternalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("INTERNAL_SECRET")
		if secret == "" {
			log.Println("CRITICAL: INTERNAL_SECRET is not set in .env! Blocking request.")
			http.Error(w, "Internal Server Error: Configuration Missing", http.StatusInternalServerError)
			return
		}

		apiKey := r.Header.Get("X-Internal-Secret")
		if apiKey != secret {
			log.Printf("[ordering-auth] WARN unauthorized internal access attempt key=%s", apiKey)
			http.Error(w, "Unauthorized: Invalid Internal Key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
