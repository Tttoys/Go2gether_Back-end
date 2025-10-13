package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// JWT configuration
	JWT JWTConfig

	// Email configuration
	Email EmailConfig

	// Google OAuth configuration
	GoogleOAuth GoogleOAuthConfig

	// CORS configuration
	CORS CORSConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxConns     int32
	MinConns     int32
	MaxLifetime  time.Duration
	ConnTimeout  time.Duration
	QueryTimeout time.Duration
}

// JWTConfig holds JWT-related configuration
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	ResetTokenTTL   time.Duration
}

// EmailConfig holds email service configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
	UseTLS       bool
	UseSSL       bool
}

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file
	if err := godotenv.Load("../.env"); err != nil {
		// Try loading from current directory if not found in parent
		if err := godotenv.Load(".env"); err != nil {
			log.Printf("Warning: .env file not found: %v", err)
		}
	}

	config := &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:     getDurationEnv("SERVER_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 5*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "5432"),
			User:         getEnv("DB_USER", "postgres"),
			Password:     getEnv("DB_PASSWORD", ""),
			Name:         getEnv("DB_NAME", "postgres"),
			SSLMode:      getEnv("DB_SSLMODE", "disable"),
			MaxConns:     getInt32Env("DB_MAX_CONNS", 5),
			MinConns:     getInt32Env("DB_MIN_CONNS", 0),
			MaxLifetime:  getDurationEnv("DB_MAX_LIFETIME", time.Hour),
			ConnTimeout:  getDurationEnv("DB_CONN_TIMEOUT", 10*time.Second),
			QueryTimeout: getDurationEnv("DB_QUERY_TIMEOUT", 30*time.Second),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessTokenTTL:  getDurationEnv("JWT_ACCESS_TTL", 7*24*time.Hour),   // 7 days
			RefreshTokenTTL: getDurationEnv("JWT_REFRESH_TTL", 30*24*time.Hour), // 30 days
			ResetTokenTTL:   getDurationEnv("JWT_RESET_TTL", 10*time.Minute),    // 10 minutes
		},
		Email: EmailConfig{
			SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:     getEnv("SMTP_PORT", "587"),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromEmail:    getEnv("EMAIL_FROM", ""),
			FromName:     getEnv("EMAIL_FROM_NAME", "Go2gether Team"),
			UseTLS:       getBoolEnv("SMTP_USE_TLS", true),
			UseSSL:       getBoolEnv("SMTP_USE_SSL", false),
		},
		GoogleOAuth: GoogleOAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		},
		CORS: CORSConfig{
			AllowedOrigins:   getStringSliceEnv("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods:   getStringSliceEnv("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders:   getStringSliceEnv("CORS_ALLOWED_HEADERS", []string{"*"}),
			AllowCredentials: getBoolEnv("CORS_ALLOW_CREDENTIALS", true),
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Check required database configuration
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}

	// Check required email configuration for production
	if c.Email.SMTPUsername == "" || c.Email.SMTPPassword == "" {
		log.Printf("Warning: SMTP credentials not configured. SMTP_USERNAME='%s', SMTP_PASSWORD='%s'. Email functionality will not work.", c.Email.SMTPUsername, c.Email.SMTPPassword)
	} else {
		log.Printf("Email configuration loaded: SMTP_HOST=%s, SMTP_PORT=%s, SMTP_USERNAME=%s", c.Email.SMTPHost, c.Email.SMTPPort, c.Email.SMTPUsername)
	}

	// Check required Google OAuth configuration
	if c.GoogleOAuth.ClientID == "" || c.GoogleOAuth.ClientSecret == "" {
		log.Println("Warning: Google OAuth credentials not configured. Google login will not work.")
	}

	return nil
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&connect_timeout=%d",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
		c.Database.SSLMode,
		int(c.Database.ConnTimeout.Seconds()),
	)
}

// IsEmailConfigured checks if email service is properly configured
func (c *Config) IsEmailConfigured() bool {
	return c.Email.SMTPUsername != "" && c.Email.SMTPPassword != "" && c.Email.FromEmail != ""
}

// IsGoogleOAuthConfigured checks if Google OAuth is properly configured
func (c *Config) IsGoogleOAuthConfigured() bool {
	return c.GoogleOAuth.ClientID != "" && c.GoogleOAuth.ClientSecret != ""
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt32Env(key string, defaultValue int32) int32 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int32(intValue)
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getStringSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		// For more complex parsing, consider using a proper CSV parser
		parts := []string{}
		for _, part := range []string{value} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return defaultValue
}
