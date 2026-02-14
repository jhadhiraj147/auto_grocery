package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwtKey stores the signing key loaded at startup via InitJWTKey.
var jwtKey []byte

// InitJWTKey loads and validates the JWT signing secret from environment configuration.
func InitJWTKey() error {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return errors.New("JWT_SECRET environment variable is not set")
	}
	jwtKey = []byte(secret)
	return nil
}

type Claims struct {
	UserID    int    `json:"user_id"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a short-lived access token for authenticated API usage.
func GenerateAccessToken(userID int, role string) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &Claims{
		UserID:    userID,
		Role:      role,
		TokenType: "ACCESS",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "auto-grocery",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// GenerateRefreshToken creates a long-lived refresh token used to issue new access tokens.
func GenerateRefreshToken(userID int, role string) (string, error) {
	expirationTime := time.Now().Add(7 * 24 * time.Hour)
	claims := &Claims{
		UserID:    userID,
		Role:      role,
		TokenType: "REFRESH",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "auto-grocery",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ValidateToken parses and verifies JWT claims and signature integrity.
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	// jwtKey is now populated from the Env var
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
