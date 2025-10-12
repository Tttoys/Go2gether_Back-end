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
