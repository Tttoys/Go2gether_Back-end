package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleOAuth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/middleware"
	"GO2GETHER_BACK-END/internal/models"
	"GO2GETHER_BACK-END/internal/utils"
)

// GoogleAuthHandler handles Google OAuth authentication
type GoogleAuthHandler struct {
	db           *pgxpool.Pool
	oauth2Config *oauth2.Config
	config       *config.Config
}

// NewGoogleAuthHandler creates a new GoogleAuthHandler instance
func NewGoogleAuthHandler(db *pgxpool.Pool, clientID, clientSecret, redirectURL string, cfg *config.Config) *GoogleAuthHandler {
	oauth2Config := &oauth2.Config{
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
		oauth2Config: oauth2Config,
		config:       cfg,
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
// @Param code query string true "Authorization code from Google"
// @Param state query string false "State parameter for CSRF protection"
// @Success 200 {object} dto.AuthResponse "Login successful"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 401 {object} dto.ErrorResponse "Invalid authorization code"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/google/callback [get]
func (h *GoogleAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authorization code from query parameters
	code := r.URL.Query().Get("code")
	_ = r.URL.Query().Get("state") // We can add state validation later if needed

	if code == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing authorization code", "Authorization code is required")
		return
	}

	// Exchange authorization code for token
	token, err := h.oauth2Config.Exchange(context.Background(), code)
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
	jwtToken, err := middleware.GenerateToken(user.ID, user.Username, user.Email, &h.config.JWT)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	// Redirect to frontend with token
	frontendURL := "http://localhost:8081/callback"
	redirectURL := fmt.Sprintf("%s?token=%s&user_id=%s&email=%s&display_name=%s&provider=%s&is_verified=%t",
		frontendURL,
		jwtToken,
		user.ID.String(),
		userInfo.Email,
		userInfo.Name,
		"google", // Since this is Google OAuth
		userInfo.Verified)

	http.Redirect(w, r, redirectURL, http.StatusFound)
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
