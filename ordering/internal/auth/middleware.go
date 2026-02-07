package auth

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey allows us to safely store data in the request context
type ContextKey string

const (
	UserKey ContextKey = "userID"
	RoleKey ContextKey = "role"
)

// AuthMiddleware is the "Security Guard"
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Get the Authorization Header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization Header", http.StatusUnauthorized)
			return
		}

		// 2. Remove "Bearer " prefix
		// Format should be: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Header Format", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// 3. Validate the Token
		claims, err := ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or Expired Token", http.StatusUnauthorized)
			return
		}

		// 4. Success! Attach User Info to the Request Context
		// This "Context" travels with the request to the next function
		ctx := context.WithValue(r.Context(), UserKey, claims.UserID)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)

		// 5. Pass the request to the real handler (with the new context)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}