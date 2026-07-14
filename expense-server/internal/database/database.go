package database

import (
	"context"
	"database/sql"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // blank import registers pgx driver
)

// New establishes a connection pool to the PostgreSQL database.
func New(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// Configure healthy connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Use a quick timeout context to ping the database
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// InitSchema reads the SQL file and executes it to update the database schema.
func InitSchema(db *sql.DB, schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(content))
	return err
}

