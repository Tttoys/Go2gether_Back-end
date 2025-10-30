// @title Go2gether Backend API
// @version 1.0
// @description Go2gether Backend API for travel companion matching
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"

	_ "GO2GETHER_BACK-END/docs" // This is required for swagger
	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/handlers"
    "GO2GETHER_BACK-END/internal/routes"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Debug: Check if email is configured
	log.Printf("Email configured: %v", cfg.IsEmailConfigured())

	// Get database connection string
	dsn := cfg.GetDSN()
	log.Println("Connecting to:", dsn)

	// ตั้งค่า pgxpool + simple protocol (จำเป็นเมื่อผ่าน PgBouncer :6543)
	dbCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	dbCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	dbCfg.ConnConfig.RuntimeParams["application_name"] = "go2gether-backend"
	dbCfg.ConnConfig.RuntimeParams["statement_timeout"] = "30000" // 30s
	dbCfg.MaxConns = cfg.Database.MaxConns
	dbCfg.MinConns = cfg.Database.MinConns
	dbCfg.MaxConnLifetime = cfg.Database.MaxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), dbCfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// ทดสอบ ping ตอนบูต
	{
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.ConnTimeout)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("ping: %v", err)
		}
	}

	// --- HTTP Handlers ---

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(pool, cfg)
	healthHandler := handlers.NewHealthHandler(pool)
    forgotPasswordHandler := handlers.NewForgotPasswordHandler(pool, cfg)
    tripsHandler := handlers.NewTripsHandler(pool, cfg)

	// Initialize Google OAuth handler
	googleAuthHandler := handlers.NewGoogleAuthHandler(pool, cfg.GoogleOAuth.ClientID, cfg.GoogleOAuth.ClientSecret, cfg.GoogleOAuth.RedirectURL, cfg)

	// Setup all routes
    routes.SetupRoutes(authHandler, healthHandler, googleAuthHandler, forgotPasswordHandler, tripsHandler, cfg)

	// --- HTTP Server + Graceful Shutdown ---
	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
	})

	// Wrap the default mux with CORS
	handler := c.Handler(http.DefaultServeMux)

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// รันเซิร์ฟเวอร์แบบ async
	go func() {
		log.Printf("HTTP server listening on :%s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// รอ SIGINT/SIGTERM เพื่อปิดอย่างสุภาพ
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped.")
}
