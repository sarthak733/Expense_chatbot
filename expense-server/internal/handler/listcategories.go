package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) ListCategories(
	ctx context.Context,
	_ *connect.Request[expensev1.ListCategoriesRequest],
) (*connect.Response[expensev1.ListCategoriesResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Retrieve user-specific categories + system-wide default categories (user_id IS NULL)
	query := `
		SELECT id, COALESCE(user_id, 0), name, COALESCE(color, ''), created_at
		FROM categories
		WHERE user_id = $1 OR user_id IS NULL
		ORDER BY user_id NULLS FIRST, name ASC`

	rows, err := h.DB.QueryContext(ctx, query, userID)
	if err != nil {
		log.Printf("ERROR listing categories: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	defer rows.Close()

	var categories []*expensev1.Category
	for rows.Next() {
		var (
			cat       expensev1.Category
			createdAt time.Time
		)
		if err := rows.Scan(&cat.Id, &cat.UserId, &cat.Name, &cat.Color, &createdAt); err != nil {
			log.Printf("ERROR scanning category row: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}
		cat.CreatedAt = formatTime(createdAt)
		categories = append(categories, &cat)
	}

	if categories == nil {
		categories = []*expensev1.Category{}
	}

	return connect.NewResponse(&expensev1.ListCategoriesResponse{
		Categories: categories,
	}), nil
}
