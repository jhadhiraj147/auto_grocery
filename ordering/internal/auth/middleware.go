package auth

import (
	"context"
	"net/http"
	"strings"
	"log"
	"os"
)

type ContextKey string

const (
	UserKey ContextKey = "userID"
	RoleKey ContextKey = "role"
)

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

func InternalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Get the secret from environment
		secret := os.Getenv("INTERNAL_SECRET")
		if secret == "" {
            // Safety Check: If env var is missing, block everything!
			log.Println("CRITICAL: INTERNAL_SECRET is not set in .env! Blocking request.")
			http.Error(w, "Internal Server Error: Configuration Missing", http.StatusInternalServerError)
			return
		}

		// 2. Check the header
		apiKey := r.Header.Get("X-Internal-Secret")
		if apiKey != secret {
			log.Printf("⚠️ Unauthorized internal access attempt. Key used: %s", apiKey)
			http.Error(w, "Unauthorized: Invalid Internal Key", http.StatusUnauthorized)
			return
		}

		// 3. Allow access
		next.ServeHTTP(w, r)
	})
}