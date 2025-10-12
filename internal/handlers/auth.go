package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/middleware"
	"GO2GETHER_BACK-END/internal/models"
	"GO2GETHER_BACK-END/internal/utils"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	db *pgxpool.Pool
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(db *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{db: db}
}

// Register handles user registration
// @Summary Register a new user
// @Description Create a new user account with username, email, and password
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "User registration data"
// @Success 201 {object} dto.AuthResponse "User created successfully"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 409 {object} dto.ErrorResponse "User already exists"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing required fields", "Username, email, and password are required")
		return
	}

	// Check if user already exists
	var existingUserID uuid.UUID
	err := h.db.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1 OR username = $2",
		req.Email, req.Username).Scan(&existingUserID)

	if err == nil {
		utils.WriteErrorResponse(w, http.StatusConflict, "User already exists", "Email or username already registered")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to hash password", err.Error())
		return
	}

	// Parse birth date if provided
	var birthDate *time.Time
	if req.BirthDate != nil && *req.BirthDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.BirthDate)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid birth date format", "Use YYYY-MM-DD format")
			return
		}
		birthDate = &parsed
	}

	// Create user
	userID := uuid.New()
	now := time.Now()

	_, err = h.db.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, username, display_name, phone, 
		 food_preferences, chronic_disease, allergic_food, allergic_drugs, 
		 emergency_contact, birth_date, role, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		userID, req.Email, string(hashedPassword), req.Username, req.DisplayName, req.Phone,
		req.FoodPreferences, req.ChronicDisease, req.AllergicFood, req.AllergicDrugs,
		req.EmergencyContact, birthDate, "user", now, now)

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to create user", err.Error())
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(userID, req.Username, req.Email)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	// Create user object for response
	user := models.User{
		ID:               userID,
		Email:            req.Email,
		Username:         req.Username,
		DisplayName:      req.DisplayName,
		Phone:            req.Phone,
		FoodPreferences:  req.FoodPreferences,
		ChronicDisease:   req.ChronicDisease,
		AllergicFood:     req.AllergicFood,
		AllergicDrugs:    req.AllergicDrugs,
		EmergencyContact: req.EmergencyContact,
		BirthDate:        birthDate,
		Role:             "user",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

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
		BirthDate:        req.BirthDate,
		Role:             user.Role,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        user.UpdatedAt.Format(time.RFC3339),
	}

	response := dto.AuthResponse{
		User:  userResponse,
		Token: token,
	}

	utils.WriteJSONResponse(w, http.StatusCreated, response)
}

// Login handles user login
// @Summary Login user
// @Description Authenticate user with email and password
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.AuthResponse "Login successful"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 401 {object} dto.ErrorResponse "Invalid credentials"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing required fields", "Email and password are required")
		return
	}

	// Get user from database
	var user models.User
	err := h.db.QueryRow(context.Background(),
		`SELECT id, email, password_hash, username, display_name, phone, 
		 food_preferences, chronic_disease, allergic_food, allergic_drugs, 
		 emergency_contact, activities, food_categories, birth_date, role, 
		 created_at, updated_at FROM users WHERE email = $1`,
		req.Email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username,
		&user.DisplayName, &user.Phone, &user.FoodPreferences, &user.ChronicDisease,
		&user.AllergicFood, &user.AllergicDrugs, &user.EmergencyContact, &user.Activities,
		&user.FoodCategories, &user.BirthDate, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Email or password is incorrect")
		return
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", "Email or password is incorrect")
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(user.ID, user.Username, user.Email)
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
		BirthDate: func() *string {
			if user.BirthDate != nil {
				s := user.BirthDate.Format("2006-01-02")
				return &s
			} else {
				return nil
			}
		}(),
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}

	response := dto.AuthResponse{
		User:  userResponse,
		Token: token,
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// GetProfile returns the current user's profile
// @Summary Get user profile
// @Description Get the current authenticated user's profile information
// @Tags authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.UserResponse "User profile retrieved successfully"
// @Failure 401 {object} dto.ErrorResponse "Unauthorized"
// @Failure 404 {object} dto.ErrorResponse "User not found"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/profile [get]
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from context (set by AuthMiddleware)
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	// Get user from database
	var user models.User
	err := h.db.QueryRow(context.Background(),
		`SELECT id, email, username, display_name, phone, 
		 food_preferences, chronic_disease, allergic_food, allergic_drugs, 
		 emergency_contact, activities, food_categories, birth_date, role, 
		 created_at, updated_at FROM users WHERE id = $1`,
		userID).Scan(&user.ID, &user.Email, &user.Username, &user.DisplayName,
		&user.Phone, &user.FoodPreferences, &user.ChronicDisease, &user.AllergicFood,
		&user.AllergicDrugs, &user.EmergencyContact, &user.Activities,
		&user.FoodCategories, &user.BirthDate, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "User not found", err.Error())
		return
	}

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
		BirthDate: func() *string {
			if user.BirthDate != nil {
				s := user.BirthDate.Format("2006-01-02")
				return &s
			} else {
				return nil
			}
		}(),
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}

	utils.WriteJSONResponse(w, http.StatusOK, userResponse)
}
