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

func (h *Handler) UpdateExpense(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateExpenseRequest],
) (*connect.Response[expensev1.UpdateExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.Title == "" || req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title and amount (>0) are required"))
	}

	catID, catName, err := h.resolveCategory(ctx, userID, req.Msg.CategoryId, req.Msg.Category)
	if err != nil {
		log.Printf("ERROR resolving category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	query := `
		UPDATE expenses
		SET title = $1, amount = $2, category = $3, category_id = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, title, amount, category, category_id, created_at`

	var (
		exp       expensev1.Expense
		createdAt time.Time
	)
	err = h.DB.QueryRowContext(ctx, query,
		req.Msg.Title, req.Msg.Amount, catName, catID, req.Msg.Id, userID,
	).Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("expense not found"))
		}
		log.Printf("ERROR updating expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	exp.CreatedAt = formatTime(createdAt)

	return connect.NewResponse(&expensev1.UpdateExpenseResponse{
		Message: "expense updated successfully",
		Expense: &exp,
	}), nil
}
