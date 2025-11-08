package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/utils"
)

// JWTClaims represents the claims in the JWT token
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken generates a JWT token for the given user
func GenerateToken(userID uuid.UUID, email string, cfg *config.JWTConfig) (string, error) {
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string, cfg *config.JWTConfig) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrTokenMalformed
}

// InvitationTokenClaims represents the JWT claims for trip invitation link
type InvitationTokenClaims struct {
	TripID uuid.UUID `json:"trip_id"`
	jwt.RegisteredClaims
}

// GenerateInvitationToken generates a JWT token for trip invitation link
func GenerateInvitationToken(tripID uuid.UUID, cfg *config.JWTConfig) (string, error) {
	claims := InvitationTokenClaims{
		TripID: tripID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)), // 30 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   "trip_invitation",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ValidateInvitationToken validates and parses the invitation token
func ValidateInvitationToken(tokenString string, cfg *config.JWTConfig) (*InvitationTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &InvitationTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*InvitationTokenClaims); ok && token.Valid {
		if claims.Subject != "trip_invitation" {
			return nil, jwt.ErrTokenMalformed
		}
		return claims, nil
	}

	return nil, jwt.ErrTokenMalformed
}

// AuthMiddleware validates JWT tokens in the Authorization header
func AuthMiddleware(next http.HandlerFunc, cfg *config.JWTConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
            utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Authorization header required")
			return
		}

		// Extract token from "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
            utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid authorization header format")
			return
		}

		tokenString := tokenParts[1]
		claims, err := ValidateToken(tokenString, cfg)
		if err != nil {
            utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid token")
			return
		}

		// Add user info to request context
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
