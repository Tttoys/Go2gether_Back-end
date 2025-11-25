package dto

// RegisterRequest represents the request payload for user registration
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
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
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ForgotPasswordRequest represents the request to send verification code
type ForgotPasswordRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

// VerifyOTPRequest represents the request to verify OTP code
type VerifyOTPRequest struct {
	Email string `json:"email" example:"user@example.com"`
	Code  string `json:"code" example:"123456"`
}

// ResetPasswordRequest represents the request to reset password with reset token
type ResetPasswordRequest struct {
	ResetToken  string `json:"reset_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	NewPassword string `json:"new_password" example:"newPassword123"`
}

// ForgotPasswordResponse represents the response after requesting password reset
type ForgotPasswordResponse struct {
	Message   string `json:"message" example:"Verification code has been sent to your email"`
	Email     string `json:"email" example:"user@example.com"`
	ExpiresIn string `json:"expires_in" example:"3 minutes"`
}

// VerifyOTPResponse represents the response after OTP verification
type VerifyOTPResponse struct {
	Message    string `json:"message" example:"OTP verified successfully"`
	ResetToken string `json:"reset_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresIn  string `json:"expires_in" example:"10 minutes"`
}

// ResetPasswordResponse represents the response after password reset
type ResetPasswordResponse struct {
	Message string `json:"message" example:"Password has been reset successfully"`
}

// GetOTPRequest represents the request to get OTP code
type GetOTPRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

// GetOTPResponse represents the response with OTP code
type GetOTPResponse struct {
	Email     string `json:"email" example:"user@example.com"`
	Code      string `json:"code" example:"123456"`
	ExpiresAt string `json:"expires_at" example:"2025-10-27T23:42:00Z"`
	Used      bool   `json:"used" example:"false"`
	CreatedAt string `json:"created_at" example:"2025-10-27T23:39:00Z"`
}
