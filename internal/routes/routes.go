package routes

import (
	"net/http"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/handlers"
	"GO2GETHER_BACK-END/internal/middleware"

	httpSwagger "github.com/swaggo/http-swagger"
)

// SetupRoutes configures all application routes
// SetupRoutes configures all application routes
func SetupRoutes(
	authHandler *handlers.AuthHandler,
	healthHandler *handlers.HealthHandler,
	googleAuthHandler *handlers.GoogleAuthHandler,
	forgotPasswordHandler *handlers.ForgotPasswordHandler,
	tripsHandler *handlers.TripsHandler,
	profileHandler *handlers.ProfileHandler,
	cfg *config.Config,
) {
	// Health check routes
	http.HandleFunc("/healthz", healthHandler.HealthCheck)
	http.HandleFunc("/livez", healthHandler.LivenessCheck)
	http.HandleFunc("/readyz", healthHandler.ReadinessCheck)

	// Authentication routes
	http.HandleFunc("/api/auth/register", authHandler.Register)
	http.HandleFunc("/api/auth/login", authHandler.Login)
	http.HandleFunc("/api/auth/profile", middleware.AuthMiddleware(authHandler.GetProfile, &cfg.JWT))

	// Google OAuth routes
	http.HandleFunc("/api/auth/google/login", googleAuthHandler.GoogleLogin)
	http.HandleFunc("/api/auth/google/callback", googleAuthHandler.GoogleCallback)

	// Forgot Password routes
	http.HandleFunc("/api/auth/forgot-password", forgotPasswordHandler.ForgotPassword)
	http.HandleFunc("/api/auth/verify-otp", forgotPasswordHandler.VerifyOTP)
	http.HandleFunc("/api/auth/reset-password", forgotPasswordHandler.ResetPassword)
	http.HandleFunc("/api/auth/get-otp", forgotPasswordHandler.GetOTP)

	// Trip routes (GET list/POST create, and GET detail)
	http.HandleFunc("/api/trips", middleware.AuthMiddleware(tripsHandler.Trips, &cfg.JWT))
	http.HandleFunc("/api/trips/", middleware.AuthMiddleware(tripsHandler.Trips, &cfg.JWT))

	// Profile routes
	// 6.1 เพิ่มโปรไฟล์: POST /api/profile  (ต้องผ่าน AuthMiddleware เพื่อให้มี userID ใน context)
	// 6.2 GET  /api/profile  (ดูโปรไฟล์ตัวเอง)
	http.HandleFunc("/api/profile", middleware.AuthMiddleware(profileHandler.Handle, &cfg.JWT))

	// Swagger documentation (must be registered before root handler)
	http.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	// Root route with 404 handling
	http.HandleFunc("/", rootHandler)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	// If it's the root path, show welcome message
	if r.URL.Path == "/" {
		w.Write([]byte("Go2gether backend is running."))
		return
	}

	// For all other paths, return 404
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error": "Not Found", "message": "The requested resource was not found"}`))
}
