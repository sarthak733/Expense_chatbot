package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) UpdateCategory(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateCategoryRequest],
) (*connect.Response[expensev1.UpdateCategoryResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("category name is required"))
	}

	color := req.Msg.Color
	if color == "" {
		color = "#808080"
	}

	// Update only custom categories (user_id = userID). System categories cannot be updated by users.
	query := `
		UPDATE categories
		SET name = $1, color = $2
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, name, color, created_at`

	var (
		cat       expensev1.Category
		createdAt time.Time
	)
	err := h.DB.QueryRowContext(ctx, query, name, color, req.Msg.Id, userID).
		Scan(&cat.Id, &cat.UserId, &cat.Name, &cat.Color, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("category not found or cannot be modified"))
		}
		log.Printf("ERROR updating category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	cat.CreatedAt = formatTime(createdAt)

	// Also update all expenses where category_id is this category, to keep the string 'category' field in sync!
	_, _ = h.DB.ExecContext(ctx, "UPDATE expenses SET category = $1 WHERE category_id = $2 AND user_id = $3", name, cat.Id, userID)

	return connect.NewResponse(&expensev1.UpdateCategoryResponse{
		Message:  "category updated successfully",
		Category: &cat,
	}), nil
}
