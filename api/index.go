package api

import (
	"context"
	"log"
	"net/http"
	"sync"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/handlers"
	"GO2GETHER_BACK-END/internal/routes"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
)

var (
	initOnce   sync.Once
	appHandler http.Handler
	initErr    error
)

func initApp() {
	// 1) Load config (อ่านจาก ENV บน Vercel)
	cfg, err := config.Load()
	if err != nil {
		initErr = err
		log.Printf("config.Load error: %v", err)
		return
	}

	// 2) DB connection (เหมือนใน cmd/main.go)
	dsn := cfg.GetDSN()
	log.Println("Connecting to:", dsn)

	dbCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		initErr = err
		log.Printf("pgxpool.ParseConfig error: %v", err)
		return
	}
	dbCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	dbCfg.ConnConfig.RuntimeParams["application_name"] = "go2gether-backend"
	dbCfg.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
	dbCfg.MaxConns = cfg.Database.MaxConns
	dbCfg.MinConns = cfg.Database.MinConns
	dbCfg.MaxConnLifetime = cfg.Database.MaxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), dbCfg)
	if err != nil {
		initErr = err
		log.Printf("pgxpool.NewWithConfig error: %v", err)
		return
	}

	// 3) สร้าง handlers เหมือนใน main.go
	authHandler := handlers.NewAuthHandler(pool, cfg)
	healthHandler := handlers.NewHealthHandler(pool)
	forgotPasswordHandler := handlers.NewForgotPasswordHandler(pool, cfg)
	tripsHandler := handlers.NewTripsHandler(pool, cfg)
	profileHandler := handlers.NewProfileHandler(pool)
	googleAuthHandler := handlers.NewGoogleAuthHandler(
		pool,
		cfg.GoogleOAuth.ClientID,
		cfg.GoogleOAuth.ClientSecret,
		cfg.GoogleOAuth.RedirectURL,
		cfg,
	)
	notificationsHandler := handlers.NewNotificationsHandler(pool)

	// 4) Register routes ทั้งหมดลง http.DefaultServeMux
	routes.SetupRoutes(
		authHandler,
		healthHandler,
		googleAuthHandler,
		forgotPasswordHandler,
		tripsHandler,
		profileHandler,
		notificationsHandler,
		cfg,
	)

	// 5) ครอบ CORS เหมือน main.go
	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	// appHandler = CORS(default mux)
	appHandler = c.Handler(http.DefaultServeMux)

	log.Println("Vercel serverless app initialized")
}

// Handler คือ entrypoint ที่ Vercel จะเรียกทุก request
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(initApp)

	if initErr != nil {
		http.Error(w, "Server initialization error", http.StatusInternalServerError)
		return
	}
	if appHandler == nil {
		http.Error(w, "Server not ready", http.StatusInternalServerError)
		return
	}

	appHandler.ServeHTTP(w, r)
}
