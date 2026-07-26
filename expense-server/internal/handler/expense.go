package handler

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"expense-server/gen/expense/v1/expensev1connect"
)

// Compile-time proof that Handler satisfies the generated service interface.
var _ expensev1connect.ExpenseServiceHandler = (*Handler)(nil)

// formatTime converts a DB time.Time to the RFC 3339 string used in the proto.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// HELPERS FOR CATEGORY RESOLUTION
// ---------------------------------------------------------------------------

// resolveCategory retrieves or creates a category based on the input name/ID.
func (h *Handler) resolveCategory(ctx context.Context, userID int, categoryID int32, categoryName string, title string) (int32, string, error) {
	categoryName = strings.TrimSpace(categoryName)
	title = strings.TrimSpace(title)

	// Case 0: If categoryName is empty or "Others", check if this user has previously categorized an expense with this exact title.
	if (categoryID <= 0 && (categoryName == "" || categoryName == "Others")) && title != "" {
		var histCatID int32
		var histCatName string
		query := `
			SELECT category_id, category 
			FROM expenses 
			WHERE user_id = $1 AND LOWER(title) = LOWER($2) AND category_id IS NOT NULL AND category IS NOT NULL
			ORDER BY created_at DESC 
			LIMIT 1`
		err := h.DB.QueryRowContext(ctx, query, userID, title).Scan(&histCatID, &histCatName)
		if err == nil && histCatID > 0 && histCatName != "" {
			return histCatID, histCatName, nil
		}
	}

	// Case 1: category_id is explicitly provided and valid
	if categoryID > 0 {
		var name string
		// Ensure the category exists and belongs to the user or is a system default (user_id IS NULL)
		query := `SELECT id, name FROM categories WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`
		err := h.DB.QueryRowContext(ctx, query, categoryID, userID).Scan(&categoryID, &name)
		if err == nil {
			return categoryID, name, nil
		}
		if err != sql.ErrNoRows {
			return 0, "", err
		}
	}

	// Case 2: category_id was not provided or not found, but categoryName is provided
	if categoryName != "" {
		var id int32
		var name string
		// Find exact case-insensitive match (system defaults or user's custom category)
		query := `SELECT id, name FROM categories WHERE LOWER(name) = LOWER($1) AND (user_id = $2 OR user_id IS NULL) LIMIT 1`
		err := h.DB.QueryRowContext(ctx, query, categoryName, userID).Scan(&id, &name)
		if err == nil {
			return id, name, nil
		}
		if err != sql.ErrNoRows {
			return 0, "", err
		}

		// If categoryName doesn't exist, create it as a custom category for the user
		insertQuery := `INSERT INTO categories (user_id, name, color) VALUES ($1, $2, '#808080') RETURNING id, name`
		err = h.DB.QueryRowContext(ctx, insertQuery, userID, categoryName).Scan(&id, &name)
		if err == nil {
			return id, name, nil
		}
		return 0, "", err
	}

	// Case 3: Neither provided, fallback to "Others" default category
	var id int32
	var name string
	err := h.DB.QueryRowContext(ctx, "SELECT id, name FROM categories WHERE name = 'Others' AND user_id IS NULL").Scan(&id, &name)
	if err != nil {
		// Fallback absolute backup if DB is in weird state
		return 0, "Others", nil
	}
	return id, name, nil
}
