// สร้างไฟล์ internal/handlers/forgot_password.go

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/middleware"
	"GO2GETHER_BACK-END/internal/utils"
)

// ForgotPasswordHandler handles forgot password functionality
type ForgotPasswordHandler struct {
	db *pgxpool.Pool
}

// NewForgotPasswordHandler creates a new ForgotPasswordHandler instance
func NewForgotPasswordHandler(db *pgxpool.Pool) *ForgotPasswordHandler {
	return &ForgotPasswordHandler{db: db}
}

// ForgotPassword sends verification code to user's email
// @Summary Request password reset
// @Description Send 6-digit verification code to user's email for password reset
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address"
// @Success 200 {object} dto.ForgotPasswordResponse "Verification code sent successfully"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 404 {object} dto.ErrorResponse "User not found"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/forgot-password [post]
func (h *ForgotPasswordHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate email
	if req.Email == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing required field", "Email is required")
		return
	}

	// Check if user exists
	var userID uuid.UUID
	err := h.db.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", req.Email).Scan(&userID)

	if err != nil {
		if err == pgx.ErrNoRows {
			utils.WriteErrorResponse(w, http.StatusNotFound, "User not found", "No account found with this email")
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Check if there's a recent unused code (within 3 minutes)
	var existingCode string
	var expiresAt time.Time
	err = h.db.QueryRow(context.Background(),
		`SELECT code, expires_at FROM auth_verifications 
		 WHERE user_id = $1 AND used = false AND expires_at > NOW()
		 ORDER BY created_at DESC LIMIT 1`,
		userID).Scan(&existingCode, &expiresAt)

	if err == nil {
		// There's still a valid code, check if it's within cooldown period
		timeRemaining := time.Until(expiresAt)
		if timeRemaining > 0 {
			utils.WriteErrorResponse(w, http.StatusTooManyRequests,
				"Code already sent",
				fmt.Sprintf("Please wait %d seconds before requesting a new code", int(timeRemaining.Seconds())))
			return
		}
	}

	// Generate 6-digit verification code
	code, err := generateVerificationCode(6)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate code", err.Error())
		return
	}

	// Store verification code in database (expires in 3 minutes)
	expiresAt = time.Now().Add(3 * time.Minute)
	_, err = h.db.Exec(context.Background(),
		`INSERT INTO auth_verifications (user_id, email, code, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, req.Email, code, expiresAt, time.Now())

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to store verification code", err.Error())
		return
	}

	// TODO: Send verification code via email service
	// For development, log the code
	fmt.Printf("Verification code for %s: %s (expires in 3 minutes)\n", req.Email, code)

	response := dto.ForgotPasswordResponse{
		Message:   "Verification code has been sent to your email",
		Email:     req.Email,
		ExpiresIn: "3 minutes",
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// VerifyOTP verifies the OTP and returns a reset token
// @Summary Verify OTP
// @Description Verify the 6-digit code and get a temporary reset token
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.VerifyOTPRequest true "Email and verification code"
// @Success 200 {object} dto.VerifyOTPResponse "OTP verified successfully"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 401 {object} dto.ErrorResponse "Invalid or expired code"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/verify-otp [post]
func (h *ForgotPasswordHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate required fields
	if req.Email == "" || req.Code == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing required fields", "Email and code are required")
		return
	}

	// Get user ID
	var userID uuid.UUID
	err := h.db.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", req.Email).Scan(&userID)

	if err != nil {
		if err == pgx.ErrNoRows {
			utils.WriteErrorResponse(w, http.StatusNotFound, "User not found", "No account found with this email")
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Verify code
	var verificationID uuid.UUID
	var storedCode string
	var expiresAt time.Time
	var used bool
	err = h.db.QueryRow(context.Background(),
		`SELECT id, code, expires_at, used FROM auth_verifications 
		 WHERE user_id = $1 AND email = $2 
		 ORDER BY created_at DESC LIMIT 1`,
		userID, req.Email).Scan(&verificationID, &storedCode, &expiresAt, &used)

	if err != nil {
		if err == pgx.ErrNoRows {
			utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid code", "No verification code found")
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Check if code has been used
	if used {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Code already used", "This verification code has already been used")
		return
	}

	// Check if code has expired
	if time.Now().After(expiresAt) {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Code expired", "Verification code has expired. Please request a new one")
		return
	}

	// Check if code matches
	if storedCode != req.Code {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid code", "The verification code you entered is incorrect")
		return
	}

	// Generate reset token (valid for 10 minutes)
	resetToken, err := middleware.GenerateResetToken(userID, req.Email, req.Code)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate reset token", err.Error())
		return
	}

	response := dto.VerifyOTPResponse{
		Message:    "OTP verified successfully",
		ResetToken: resetToken,
		ExpiresIn:  "10 minutes",
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// ResetPassword resets user's password using reset token
// @Summary Reset password
// @Description Reset user's password with new password using reset token
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} dto.ResetPasswordResponse "Password reset successfully"
// @Failure 400 {object} dto.ErrorResponse "Invalid request data"
// @Failure 401 {object} dto.ErrorResponse "Invalid or expired reset token"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Router /api/auth/reset-password [post]
func (h *ForgotPasswordHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate required fields
	if req.ResetToken == "" || req.NewPassword == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Missing required fields", "Reset token and new password are required")
		return
	}

	// Validate password length
	if len(req.NewPassword) < 6 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Password too short", "Password must be at least 6 characters long")
		return
	}

	// Validate reset token
	claims, err := middleware.ValidateResetToken(req.ResetToken)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid reset token", err.Error())
		return
	}

	// Check if the verification code is still valid and not used
	var verificationID uuid.UUID
	var used bool
	var expiresAt time.Time
	err = h.db.QueryRow(context.Background(),
		`SELECT id, used, expires_at FROM auth_verifications 
		 WHERE user_id = $1 AND email = $2 AND code = $3
		 ORDER BY created_at DESC LIMIT 1`,
		claims.UserID, claims.Email, claims.Code).Scan(&verificationID, &used, &expiresAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid verification", "No matching verification found")
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Check if code has been used
	if used {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Code already used", "This verification code has already been used")
		return
	}

	// Check if code has expired
	if time.Now().After(expiresAt) {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Code expired", "Verification code has expired")
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to hash password", err.Error())
		return
	}

	// Start transaction
	tx, err := h.db.Begin(context.Background())
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback(context.Background())

	// Update user's password
	_, err = tx.Exec(context.Background(),
		`UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`,
		string(hashedPassword), time.Now(), claims.UserID)

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to update password", err.Error())
		return
	}

	// Mark verification code as used
	_, err = tx.Exec(context.Background(),
		"UPDATE auth_verifications SET used = true WHERE id = $1", verificationID)

	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to mark code as used", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit(context.Background()); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to commit transaction", err.Error())
		return
	}

	response := dto.ResetPasswordResponse{
		Message: "Password has been reset successfully",
	}

	utils.WriteJSONResponse(w, http.StatusOK, response)
}

// generateVerificationCode generates a random n-digit verification code
func generateVerificationCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)

	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}

	return string(code), nil
}
