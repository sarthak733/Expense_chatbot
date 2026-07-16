package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) CreateCategory(
	ctx context.Context,
	req *connect.Request[expensev1.CreateCategoryRequest],
) (*connect.Response[expensev1.CreateCategoryResponse], error) {

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

	// Insert custom category
	query := `
		INSERT INTO categories (user_id, name, color)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err := h.DB.QueryRowContext(ctx, query, userID, name, color).Scan(&id, &createdAt)
	if err != nil {
		// Category might already exist
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("category with this name already exists"))
	}

	return connect.NewResponse(&expensev1.CreateCategoryResponse{
		Message: "category created successfully",
		Category: &expensev1.Category{
			Id:        id,
			UserId:    int32(userID),
			Name:      name,
			Color:     color,
			CreatedAt: formatTime(createdAt),
		},
	}), nil
}
