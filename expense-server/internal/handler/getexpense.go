package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) GetExpense(
	ctx context.Context,
	req *connect.Request[expensev1.GetExpenseRequest],
) (*connect.Response[expensev1.GetExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT id, user_id, title, amount, COALESCE(category, ''), COALESCE(category_id, 0), created_at, currency
		FROM expenses
		WHERE id = $1 AND user_id = $2`

	var (
		exp       expensev1.Expense
		createdAt time.Time
	)
	err := h.DB.QueryRowContext(ctx, query, req.Msg.Id, userID).
		Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt, &exp.Currency)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("expense not found"))
		}
		log.Printf("ERROR fetching expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	exp.CreatedAt = formatTime(createdAt)

	return connect.NewResponse(&expensev1.GetExpenseResponse{
		Message: "expense retrieved successfully",
		Expense: &exp,
	}), nil
}
