package routes

import (
	"net/http"

	"GO2GETHER_BACK-END/internal/handlers"
	"GO2GETHER_BACK-END/internal/middleware"
)

// SetupRoutes configures all application routes
func SetupRoutes(authHandler *handlers.AuthHandler, healthHandler *handlers.HealthHandler) {
	// Health check routes
	http.HandleFunc("/healthz", healthHandler.HealthCheck)
	http.HandleFunc("/livez", healthHandler.LivenessCheck)
	http.HandleFunc("/readyz", healthHandler.ReadinessCheck)

	// Authentication routes
	http.HandleFunc("/api/auth/register", authHandler.Register)
	http.HandleFunc("/api/auth/login", authHandler.Login)
	http.HandleFunc("/api/auth/profile", middleware.AuthMiddleware(authHandler.GetProfile))

	// Root route
	http.HandleFunc("/", rootHandler)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Go2gether backend is running."))
}
