package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleOAuth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/middleware"
	"GO2GETHER_BACK-END/internal/models"
	"GO2GETHER_BACK-END/internal/utils"
)

// GoogleAuthHandler handles Google OAuth authentication
type GoogleAuthHandler struct {
	db           *pgxpool.Pool
	oauth2Config *oauth2.Config
}

// NewGoogleAuthHandler creates a new GoogleAuthHandler instance
func NewGoogleAuthHandler(db *pgxpool.Pool, clientID, clientSecret, redirectURL string) *GoogleAuthHandler {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleAuthHandler{
		db:           db,
		oauth2Config: config,
	}
}

// GoogleLogin initiates Google OAuth login
// @Summary Google OAuth login
// @Description Initiate Google OAuth login flow
// @Tags authentication
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "Google OAuth URL"
// @Router /api/auth/google/login [get]
func (h *GoogleAuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate state parameter for CSRF protection
	state := uuid.New().String()

	// Create the authorization URL
	authURL := h.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	response := map[string]string{
		"auth_url": authURL,
		"state":    state,
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// GoogleCallback handles Google OAuth callback
// @Summary Google OAuth callback
// @Description Handle Google OAuth callback with authorization code
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body GoogleLoginRequest true "Google OAuth code"
// @Success 200 {object} models.AuthResponse "Login successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request data"
// @Failure 401 {object} models.ErrorResponse "Invalid authorization code"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/auth/google/callback [post]
func (h *GoogleAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.GoogleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Code == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing authorization code", "Authorization code is required")
		return
	}

	// Exchange authorization code for token
	token, err := h.oauth2Config.Exchange(context.Background(), req.Code)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid authorization code", err.Error())
		return
	}

	// Get user info from Google
	userInfo, err := h.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get user info", err.Error())
		return
	}

	// Check if user exists in database
	var user models.User
	err = h.db.QueryRow(context.Background(),
		`SELECT id, email, password_hash, username, display_name, phone, 
		 food_preferences, chronic_disease, allergic_food, allergic_drugs, 
		 emergency_contact, activities, food_categories, birth_date, role, 
		 created_at, updated_at FROM users WHERE email = $1`,
		userInfo.Email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username,
		&user.DisplayName, &user.Phone, &user.FoodPreferences, &user.ChronicDisease,
		&user.AllergicFood, &user.AllergicDrugs, &user.EmergencyContact, &user.Activities,
		&user.FoodCategories, &user.BirthDate, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		// User doesn't exist, create new user
		user, err = h.createGoogleUser(userInfo)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to create user", err.Error())
			return
		}
	}

	// Generate JWT token
	jwtToken, err := middleware.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	// Clear password from response
	user.PasswordHash = ""

	// Convert user to DTO
	userResponse := dto.UserResponse{
		ID:               user.ID.String(),
		Email:            user.Email,
		Username:         user.Username,
		DisplayName:      user.DisplayName,
		Phone:            user.Phone,
		FoodPreferences:  user.FoodPreferences,
		ChronicDisease:   user.ChronicDisease,
		AllergicFood:     user.AllergicFood,
		AllergicDrugs:    user.AllergicDrugs,
		EmergencyContact: user.EmergencyContact,
		Activities:       user.Activities,
		FoodCategories:   user.FoodCategories,
		BirthDate:        &[]string{user.BirthDate.Format("2006-01-02")}[0],
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
	}

	response := dto.AuthResponse{
		User:  userResponse,
		Token: jwtToken,
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// getGoogleUserInfo fetches user information from Google
func (h *GoogleAuthHandler) getGoogleUserInfo(accessToken string) (*dto.GoogleUserInfo, error) {
	ctx := context.Background()
	service, err := googleOAuth2.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})))
	if err != nil {
		return nil, err
	}

	userInfo, err := service.Userinfo.Get().Do()
	if err != nil {
		return nil, err
	}

	verified := false
	if userInfo.VerifiedEmail != nil {
		verified = *userInfo.VerifiedEmail
	}

	return &dto.GoogleUserInfo{
		ID:       userInfo.Id,
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Picture:  userInfo.Picture,
		Verified: verified,
	}, nil
}

// createGoogleUser creates a new user from Google OAuth data
func (h *GoogleAuthHandler) createGoogleUser(googleUser *dto.GoogleUserInfo) (models.User, error) {
	userID := uuid.New()
	now := time.Now()

	// Generate a random username from email
	username := googleUser.Email
	if len(username) > 50 {
		username = username[:50]
	}

	_, err := h.db.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, username, display_name, avatar_url, role, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		userID, googleUser.Email, "", username, &googleUser.Name, &googleUser.Picture, "user", now, now)

	if err != nil {
		return models.User{}, err
	}

	return models.User{
		ID:          userID,
		Email:       googleUser.Email,
		Username:    username,
		DisplayName: &googleUser.Name,
		AvatarURL:   &googleUser.Picture,
		Role:        "user",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
