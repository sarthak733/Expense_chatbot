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

	catID, catName, err := h.resolveCategory(ctx, userID, req.Msg.CategoryId, req.Msg.Category, req.Msg.Title)
	if err != nil {
		log.Printf("ERROR resolving category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	currency := strings.TrimSpace(req.Msg.Currency)

	var query string
	var args []interface{}

	if currency != "" {
		query = `
			UPDATE expenses
			SET title = $1, amount = $2, category = $3, category_id = $4, currency = $5
			WHERE id = $6 AND user_id = $7
			RETURNING id, user_id, title, amount, category, category_id, created_at, currency`
		args = []interface{}{req.Msg.Title, req.Msg.Amount, catName, catID, currency, req.Msg.Id, userID}
	} else {
		query = `
			UPDATE expenses
			SET title = $1, amount = $2, category = $3, category_id = $4
			WHERE id = $5 AND user_id = $6
			RETURNING id, user_id, title, amount, category, category_id, created_at, currency`
		args = []interface{}{req.Msg.Title, req.Msg.Amount, catName, catID, req.Msg.Id, userID}
	}

	var (
		exp       expensev1.Expense
		createdAt time.Time
	)
	err = h.DB.QueryRowContext(ctx, query, args...).Scan(
		&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt, &exp.Currency,
	)

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
