package models

import (
	"time"

	"github.com/google/uuid"
)

// Profile represents a user profile in the system
// This is kept for potential future use
type Profile struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
