package main

import (
	"context"
	"fmt"
	"os"

	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/coding-cave-dev/nimbul/internal/httpserver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func getDatabaseURL() string {
	// Check if DATABASE_URL is set directly
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		return databaseURL
	}

	// Construct database URL from individual PostgreSQL environment variables
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "nimbul"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "nimbul"
	}
	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "nimbul"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbName)
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		// Don't panic if .env file doesn't exist, use system env vars
	}

	// Initialize database connection
	databaseURL := getDatabaseURL()

	conn, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Create queries instance
	queries := db.New(conn)

	// Initialize router with database queries
	router := httpserver.NewRouter(queries)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Listen(":" + port); err != nil {
		panic(err)
	}
}
