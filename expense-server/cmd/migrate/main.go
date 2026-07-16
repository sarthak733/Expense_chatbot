package main

import (
	"context"
	"log"
	"os"

	"ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	entmigrate "expense-server/ent/migrate"

	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

	// 1. Create a local migration directory
	dir, err := migrate.NewLocalDir("migrations")
	if err != nil {
		log.Fatalf("failed creating atlas migration directory: %v", err)
	}

	// 2. Configure migration options
	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(migrate.DefaultFormatter),
	}

	// 3. Setup migration name
	name := "init_schema"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	// 4. PostgreSQL Dev URL (matches dev database we created)
	devURL := "postgres://postgres:sergtsop@localhost:5432/expense_tracker_dev?sslmode=disable"
	if envURL := os.Getenv("DEV_DATABASE_URL"); envURL != "" {
		devURL = envURL
	}

	// 5. Generate migration diff
	err = entmigrate.NamedDiff(ctx, devURL, name, opts...)
	if err != nil {
		log.Fatalf("failed generating migration: %v", err)
	}

	log.Printf("Successfully generated migration files with name '%s' in migrations/ directory.\n", name)
}
