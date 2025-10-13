// สร้างไฟล์ internal/middleware/reset_token.go

package middleware

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ResetTokenClaims represents the JWT claims for password reset token
type ResetTokenClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Code   string    `json:"code"`
	jwt.RegisteredClaims
}

// GenerateResetToken generates a temporary JWT token for password reset
func GenerateResetToken(userID uuid.UUID, email, code string) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production"
	}

	// Token expires in 10 minutes
	expirationTime := time.Now().Add(10 * time.Minute)

	claims := &ResetTokenClaims{
		UserID: userID,
		Email:  email,
		Code:   code,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "go2gether",
			Subject:   "password_reset",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateResetToken validates and parses the reset token
func ValidateResetToken(tokenString string) (*ResetTokenClaims, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production"
	}

	token, err := jwt.ParseWithClaims(tokenString, &ResetTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*ResetTokenClaims); ok && token.Valid {
		// Check if token is for password reset
		if claims.Subject != "password_reset" {
			return nil, errors.New("invalid token type")
		}

		// Check if token has expired
		if claims.ExpiresAt.Before(time.Now()) {
			return nil, errors.New("token has expired")
		}

		return claims, nil
	}

	return nil, errors.New("invalid token")
}
