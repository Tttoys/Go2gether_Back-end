package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type Profile struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Email            string     `json:"email" db:"email"`
	PasswordHash     string     `json:"-" db:"password_hash"` // Hidden from JSON responses
	Username         string     `json:"username" db:"username"`
	DisplayName      *string    `json:"display_name" db:"display_name"`
	AvatarURL        *string    `json:"avatar_url" db:"avatar_url"`
	Phone            *string    `json:"phone" db:"phone"`
	FoodPreferences  *string    `json:"food_preferences" db:"food_preferences"`
	ChronicDisease   *string    `json:"chronic_disease" db:"chronic_disease"`
	AllergicFood     *string    `json:"allergic_food" db:"allergic_food"`
	AllergicDrugs    *string    `json:"allergic_drugs" db:"allergic_drugs"`
	EmergencyContact *string    `json:"emergency_contact" db:"emergency_contact"`
	Activities       *string    `json:"activities" db:"activities"`           // JSONB as string
	FoodCategories   *string    `json:"food_categories" db:"food_categories"` // Array as string
	BirthDate        *time.Time `json:"birth_date" db:"birth_date"`
	Role             string     `json:"role" db:"role"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// RegisterRequest represents the request payload for user registration
type RegisterRequest struct {
	Username         string  `json:"username" validate:"required,min=3,max=50"`
	Email            string  `json:"email" validate:"required,email"`
	Password         string  `json:"password" validate:"required,min=6"`
	DisplayName      *string `json:"display_name,omitempty"`
	Phone            *string `json:"phone,omitempty"`
	FoodPreferences  *string `json:"food_preferences,omitempty"`
	ChronicDisease   *string `json:"chronic_disease,omitempty"`
	AllergicFood     *string `json:"allergic_food,omitempty"`
	AllergicDrugs    *string `json:"allergic_drugs,omitempty"`
	EmergencyContact *string `json:"emergency_contact,omitempty"`
	BirthDate        *string `json:"birth_date,omitempty"` // Will be parsed to time.Time
}

// LoginRequest represents the request payload for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
