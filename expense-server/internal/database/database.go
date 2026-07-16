package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// ApplyMigrations runs versioned SQL migrations in alphabetical order,
// tracking applied migrations in a schema_migrations table.
func ApplyMigrations(db *sql.DB, migrationsDir string) error {
	// Create schema_migrations table if not exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check if this is a transition from the old unversioned DB to versioned migrations.
	// If the "users" table already exists but we have no records in "schema_migrations",
	// we will automatically baseline the initial schema migration so it doesn't fail on creation.
	var usersTableExists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users')").Scan(&usersTableExists)

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)

	isTransition := usersTableExists && count == 0

	// Read migrations directory
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// We assume the first migration file alphabetically is the baseline migration.
	var firstMigration string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			firstMigration = entry.Name()
			break
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Check if version was already applied
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", entry.Name()).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration state: %w", err)
		}

		if exists {
			continue
		}

		if isTransition && entry.Name() == firstMigration {
			// Record the migration as applied without executing the DDL, to baseline the database
			_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", entry.Name())
			if err != nil {
				return fmt.Errorf("failed to baseline migration %s: %w", entry.Name(), err)
			}
			log.Printf("baselined initial migration (users table already exists): %s\n", entry.Name())
			continue
		}

		// Read and execute SQL file
		filePath := filepath.Join(migrationsDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		// Execute migration within a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		_, err = tx.Exec(string(content))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", entry.Name(), err)
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", entry.Name())
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration transaction %s: %w", entry.Name(), err)
		}

		log.Printf("applied migration: %s\n", entry.Name())
	}

	return nil
}

// SeedDefaultCategories inserts system categories if they don't exist yet.
func SeedDefaultCategories(db *sql.DB) error {
	defaults := []struct {
		Name  string
		Color string
	}{
		{"Food", "#FF5733"},
		{"Transport", "#3357FF"},
		{"Utilities", "#33FF57"},
		{"Entertainment", "#F033FF"},
		{"Shopping", "#FF33A6"},
		{"Others", "#808080"},
	}

	for _, d := range defaults {
		query := `
			INSERT INTO categories (user_id, name, color, created_at)
			SELECT NULL, $1, $2, NOW()
			WHERE NOT EXISTS (
				SELECT 1 FROM categories WHERE user_id IS NULL AND name = $3
			)`
		_, err := db.Exec(query, d.Name, d.Color, d.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

