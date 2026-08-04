package scratch

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDB() {
	passwords := []string{"postgres", "sergtsop", "admin", "root", ""}
	dbName := "expense_tracker" // we will also try default "postgres" first to see if authentication succeeds

	for _, pw := range passwords {
		// First try connecting to default "postgres" database just to check password auth
		dsn := fmt.Sprintf("postgres://postgres:%s@localhost:5432/postgres?sslmode=disable", pw)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			continue
		}
		
		err = db.Ping()
		db.Close()
		
		if err == nil {
			fmt.Printf("[SUCCESS] Connected to 'postgres' database with password: '%s'\n", pw)
			
			// Now check if 'expense_tracker' database exists
			testDsn := fmt.Sprintf("postgres://postgres:%s@localhost:5432/%s?sslmode=disable", pw, dbName)
			testDb, testErr := sql.Open("pgx", testDsn)
			if testErr == nil {
				pingErr := testDb.Ping()
				testDb.Close()
				if pingErr == nil {
					fmt.Printf("[SUCCESS] Connected to '%s' database with password: '%s'\n", dbName, pw)
				} else {
					fmt.Printf("[INFO] Password '%s' is correct, but could not connect to '%s' database: %v\n", pw, dbName, pingErr)
				}
			}
			return
		} else {
			fmt.Printf("[FAILED] Password '%s' failed: %v\n", pw, err)
		}
	}
	fmt.Println("[ERROR] Could not connect with any common default passwords. Please specify your password.")
}
