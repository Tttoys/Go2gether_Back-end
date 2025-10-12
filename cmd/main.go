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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/cors"

	_ "GO2GETHER_BACK-END/docs" // This is required for swagger
	"GO2GETHER_BACK-END/internal/handlers"
	"GO2GETHER_BACK-END/internal/routes"
)

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}

func main() {
	// โหลด .env (ถ้า main.go อยู่ใน cmd/ และ .env อยู่ที่ root ใช้ "../.env")
	if err := godotenv.Load("../.env"); err != nil {
		// ถ้าไม่เจอ .env ที่ root ลองหาใน current directory
		if err := godotenv.Load(".env"); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	// สร้าง DSN จากค่าที่แยกใน .env
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&connect_timeout=10",
		mustEnv("DB_USER"),
		mustEnv("DB_PASSWORD"),
		mustEnv("DB_HOST"),
		mustEnv("DB_PORT"),
		mustEnv("DB_NAME"),    // แนะนำให้เป็น "postgres" สำหรับ Supabase
		mustEnv("DB_SSLMODE"), // "require"
	)
	fmt.Println("Connecting to:", dsn)

	// ตั้งค่า pgxpool + simple protocol (จำเป็นเมื่อผ่าน PgBouncer :6543)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.RuntimeParams["application_name"] = "go2gether-backend"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "30000" // 30s
	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// ทดสอบ ping ตอนบูต
	{
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("ping: %v", err)
		}
	}

	// --- HTTP Handlers ---

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(pool)
	healthHandler := handlers.NewHealthHandler(pool)

	// Initialize Google OAuth handler
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		googleRedirectURL = "http://localhost:8080/api/auth/google/callback"
	}
	googleAuthHandler := handlers.NewGoogleAuthHandler(pool, googleClientID, googleClientSecret, googleRedirectURL)

	// Setup all routes
	routes.SetupRoutes(authHandler, healthHandler, googleAuthHandler)

	// --- HTTP Server + Graceful Shutdown ---
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins for development
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	// Wrap the default mux with CORS
	handler := c.Handler(http.DefaultServeMux)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// รันเซิร์ฟเวอร์แบบ async
	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// รอ SIGINT/SIGTERM เพื่อปิดอย่างสุภาพ
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stoppeded.")
}
