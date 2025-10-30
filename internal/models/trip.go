package models

import (
	"time"

	"github.com/google/uuid"
)

// Trip represents a travel trip created by a user
type Trip struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Destination string    `json:"destination" db:"destination"`
	StartDate   time.Time `json:"start_date" db:"start_date"`
	EndDate     time.Time `json:"end_date" db:"end_date"`
	Description string    `json:"description" db:"description"`
	Status      string    `json:"status" db:"status"`
	TotalBudget float64   `json:"total_budget" db:"total_budget"`
	Currency    string    `json:"currency" db:"currency"`
	CreatorID   uuid.UUID `json:"creator_id" db:"creator_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
