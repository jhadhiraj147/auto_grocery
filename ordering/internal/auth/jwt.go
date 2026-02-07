package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// In a real app, this comes from os.Getenv("JWT_SECRET")
var jwtKey = []byte("super_secret_access_key_123")

// The "Claims" is the data inside the QR Code/Token
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"` // "CLIENT" or "TRUCK"
	jwt.RegisteredClaims
}

// 1. Generate Access Token (15 Minutes)
func GenerateAccessToken(userID int, role string) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute)
	
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "auto-grocery",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// 2. Generate Refresh Token (7 Days)
func GenerateRefreshToken(userID int, role string) (string, error) {
	expirationTime := time.Now().Add(7 * 24 * time.Hour)
	
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "auto-grocery",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// 3. Validate Token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

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