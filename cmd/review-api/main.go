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

func main() {
	store := mustBuildStore()
	service := review.NewService(store)
	handler := api.NewHandler(service)

	addr := ":" + envOrDefault("PORT", "8080")
	log.Printf("review-api listening on %s", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

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

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
