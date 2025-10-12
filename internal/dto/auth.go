package dto

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
	User  UserResponse `json:"user"`
	Token string       `json:"token"`
}

// UserResponse represents user data in API responses
type UserResponse struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Username         string  `json:"username"`
	DisplayName      *string `json:"display_name"`
	AvatarURL        *string `json:"avatar_url"`
	Phone            *string `json:"phone"`
	FoodPreferences  *string `json:"food_preferences"`
	ChronicDisease   *string `json:"chronic_disease"`
	AllergicFood     *string `json:"allergic_food"`
	AllergicDrugs    *string `json:"allergic_drugs"`
	EmergencyContact *string `json:"emergency_contact"`
	Activities       *string `json:"activities"`
	FoodCategories   *string `json:"food_categories"`
	BirthDate        *string `json:"birth_date,omitempty"`
	Role             string  `json:"role"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
