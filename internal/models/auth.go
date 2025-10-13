package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
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
	FoodCategories   []string   `json:"food_categories" db:"food_categories"` // Array as string
	BirthDate        *time.Time `json:"birth_date" db:"birth_date"`
	Role             string     `json:"role" db:"role"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}
