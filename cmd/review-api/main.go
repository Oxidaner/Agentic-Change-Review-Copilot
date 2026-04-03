package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"agentic-change-review-copilot/internal/api"
	"agentic-change-review-copilot/internal/review"
)

// main wires together the HTTP layer and the selected persistence implementation.
//
// The binary intentionally stays thin: all business behavior lives under
// internal/testflow and internal/api so the service can be tested without
// starting a real HTTP server.
func main() {
	store := mustBuildStore()
	service := review.NewService(store)
	handler := api.NewReviewHandler(service, api.WithReviewGitHubWebhookSecret(os.Getenv("GITHUB_WEBHOOK_SECRET")))

	addr := ":" + envOrDefault("PORT", "8080")
	log.Printf("review-api listening on %s", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

// mustBuildStore chooses the storage backend based on environment variables.
//
// Behavior:
// - when DATABASE_URL is missing, use the in-memory store for local/demo usage
// - when DATABASE_URL is present, connect to PostgreSQL
// - when AUTO_MIGRATE=1, execute all *.up.sql files before serving traffic
//
// The function exits the process on configuration or connectivity errors because
// the API cannot serve requests safely without a healthy storage layer.
func mustBuildStore() review.Store {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Print("DATABASE_URL not set, using in-memory store")
		return review.NewMemoryStore()
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if os.Getenv("AUTO_MIGRATE") == "1" {
		root, err := os.Getwd()
		if err != nil {
			log.Fatalf("resolve working directory: %v", err)
		}
		migrationDir := filepath.Join(root, "migrations")
		if err := review.RunMigrations(db, migrationDir); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
	}

	log.Print("using PostgreSQL store")
	return review.NewPostgresStore(db)
}

// envOrDefault provides a small helper for environment-driven configuration.
func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
