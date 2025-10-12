package dto

// GoogleLoginRequest represents the request for Google login
type GoogleLoginRequest struct {
	Code string `json:"code" validate:"required"`
}

// GoogleLoginResponse represents the response for Google login initiation
type GoogleLoginResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// GoogleUserInfo represents Google user information
type GoogleUserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
	Verified bool   `json:"verified_email"`
}
