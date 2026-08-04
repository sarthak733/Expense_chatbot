package scheduler

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

// Start runs the background scheduler that processes recurring expenses.
func Start(ctx context.Context, db *sql.DB, checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	go func() {
		// Run once immediately on startup
		RunScheduler(ctx, db)
		for {
			select {
			case <-ticker.C:
				RunScheduler(ctx, db)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// RunScheduler queries the database for recurring expenses that need to run and executes them transactionally.
func RunScheduler(ctx context.Context, db *sql.DB) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Scheduler ERROR: failed to start transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Select active recurring expenses that are due (next_run_date <= CURRENT_DATE)
	query := `
		SELECT id, user_id, title, amount, category_id, interval, next_run_date, currency
		FROM recurring_expenses
		WHERE is_active = TRUE AND next_run_date <= CURRENT_DATE
		FOR UPDATE
	`
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Scheduler ERROR: query failed: %v", err)
		return
	}
	defer rows.Close()

	type recurringJob struct {
		id          int
		userID      int
		title       string
		amount      float64
		categoryID  sql.NullInt32
		interval    string
		nextRunDate time.Time
		currency    string
	}

	var jobs []recurringJob
	for rows.Next() {
		var j recurringJob
		if err := rows.Scan(&j.id, &j.userID, &j.title, &j.amount, &j.categoryID, &j.interval, &j.nextRunDate, &j.currency); err != nil {
			log.Printf("Scheduler ERROR: scan failed: %v", err)
			return
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	if len(jobs) == 0 {
		return
	}

	today := time.Now()
	// Midnight of today in local/system time zone for comparison
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)

	for _, j := range jobs {
		nextRun := j.nextRunDate
		var lastRun time.Time

		// Catch up loop: if the server was down, generate all missed expenses
		// up to todayMidnight (inclusive)
		for !nextRun.After(todayMidnight) {
			// Find the category name
			categoryName := "Others"
			if j.categoryID.Valid {
				err = tx.QueryRowContext(ctx, "SELECT name FROM categories WHERE id = $1", j.categoryID.Int32).Scan(&categoryName)
				if err != nil {
					categoryName = "Others"
				}
			}

			// Insert expense
			// We format the date to the exact date it was supposed to run
			insertQuery := `
				INSERT INTO expenses (user_id, title, amount, category, category_id, created_at, currency)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`
			_, err = tx.ExecContext(ctx, insertQuery, j.userID, j.title, j.amount, categoryName, j.categoryID, nextRun, j.currency)
			if err != nil {
				log.Printf("Scheduler ERROR: failed to insert expense: %v", err)
				break
			}

			lastRun = nextRun

			// Calculate next run date
			switch strings.ToLower(j.interval) {
			case "daily":
				nextRun = nextRun.AddDate(0, 0, 1)
			case "weekly":
				nextRun = nextRun.AddDate(0, 0, 7)
			case "monthly":
				nextRun = nextRun.AddDate(0, 1, 0)
			case "yearly":
				nextRun = nextRun.AddDate(1, 0, 0)
			default:
				log.Printf("Scheduler ERROR: invalid interval %s for job %d, deactivating", j.interval, j.id)
				_, _ = tx.ExecContext(ctx, "UPDATE recurring_expenses SET is_active = FALSE WHERE id = $1", j.id)
				goto nextJob
			}
		}

		// Update recurring expense next_run_date and last_run_date
		if !lastRun.IsZero() {
			updateQuery := `
				UPDATE recurring_expenses
				SET last_run_date = $1, next_run_date = $2
				WHERE id = $3
			`
			_, err = tx.ExecContext(ctx, updateQuery, lastRun.Format("2006-01-02"), nextRun.Format("2006-01-02"), j.id)
			if err != nil {
				log.Printf("Scheduler ERROR: failed to update job %d: %v", j.id, err)
			} else {
				log.Printf("Scheduler: processed recurring expense %d ('%s') for user %d. Next run set to %s", 
					j.id, j.title, j.userID, nextRun.Format("2006-01-02"))
			}
		}
	nextJob:
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Scheduler ERROR: failed to commit: %v", err)
	}
}
