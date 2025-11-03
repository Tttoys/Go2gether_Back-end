package models

import "time"

// Entity ของตาราง public.profiles
type UserProfile struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	Username         string     `json:"username"`
	FirstName        *string    `json:"first_name,omitempty"`
	LastName         *string    `json:"last_name,omitempty"`
	DisplayName      *string    `json:"display_name,omitempty"`
	AvatarURL        *string    `json:"avatar_url,omitempty"`
	Phone            *string    `json:"phone,omitempty"`
	Bio              *string    `json:"bio,omitempty"`
	BirthDate        *time.Time `json:"birth_date,omitempty"`
	FoodPreferences  *string    `json:"food_preferences,omitempty"`
	ChronicDisease   *string    `json:"chronic_disease,omitempty"`
	AllergicFood     *string    `json:"allergic_food,omitempty"`
	AllergicDrugs    *string    `json:"allergic_drugs,omitempty"`
	EmergencyContact *string    `json:"emergency_contact,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
