package main

<<<<<<< HEAD
import "fmt"

func main() {
	var intnum int16 = 100
	fmt.Println(intnum)
=======
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type HealthResp struct {
	Status  string `json:"status"`
	Details any    `json:"details,omitempty"`
}

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
		log.Fatalf("Error loading .env file: %v", err)
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

	// 1) /healthz — basic health (ไม่แตะ DB)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, HealthResp{Status: "ok"})
	})

	// 2) /livez — process liveness (ยังรันอยู่/รับสัญญาณ)
	http.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, HealthResp{Status: "alive"})
	})

	// 3) /readyz — readiness (เช็คต่อ DB สำเร็จภายใน timeout)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, HealthResp{
				Status:  "degraded",
				Details: map[string]any{"db": err.Error()},
			})
			return
		}
		writeJSON(w, http.StatusOK, HealthResp{
			Status:  "ready",
			Details: map[string]any{"db": "ok"},
		})
	})

	// (ตัวอย่าง) root
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Go2gether backend is running.")
	})

	// --- HTTP Server + Graceful Shutdown ---
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
>>>>>>> main
}
