package scratch

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func ListUsers() {
	// Read .env if possible
	dbURL := "postgres://postgres:sergtsop@localhost:5432/expense_tracker?sslmode=disable"
	if content, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "DATABASE_URL=") {
				dbURL = strings.TrimPrefix(line, "DATABASE_URL=")
				dbURL = strings.Trim(dbURL, `"' ` + "\r")
				break
			}
		}
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, username, created_at FROM users")
	if err != nil {
		fmt.Printf("Error querying users: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("Registered Users:")
	count := 0
	for rows.Next() {
		var id int
		var username string
		var createdAt string
		if err := rows.Scan(&id, &username, &createdAt); err == nil {
			fmt.Printf(" - ID: %d, Username: '%s', Created At: %s\n", id, username, createdAt)
			count++
		}
	}
	if count == 0 {
		fmt.Println(" No users found in the database. You must register first!")
	}
}
