package handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) CreateExpense(
	ctx context.Context,
	req *connect.Request[expensev1.CreateExpenseRequest],
) (*connect.Response[expensev1.CreateExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.Title == "" || req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title and amount (>0) are required"))
	}

	// Resolve the category
	catID, catName, err := h.resolveCategory(ctx, userID, req.Msg.CategoryId, req.Msg.Category, req.Msg.Title)
	if err != nil {
		log.Printf("ERROR resolving category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	// Read currency from request, fallback to user's profile currency
	currency := strings.TrimSpace(req.Msg.Currency)
	if currency == "" {
		currency, _ = getUserCurrency(ctx, h.DB, userID)
	}

	query := `
		INSERT INTO expenses (user_id, title, amount, category, category_id, currency)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, currency`

	var (
		id         int
		createdAt  time.Time
		dbCurrency string
	)
	err = h.DB.QueryRowContext(ctx, query, userID, req.Msg.Title, req.Msg.Amount, catName, catID, currency).
		Scan(&id, &createdAt, &dbCurrency)
	if err != nil {
		log.Printf("ERROR inserting expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	return connect.NewResponse(&expensev1.CreateExpenseResponse{
		Message: "expense created successfully",
		Expense: &expensev1.Expense{
			Id:         int32(id),
			UserId:     int32(userID),
			Title:      req.Msg.Title,
			Amount:     req.Msg.Amount,
			Category:   catName,
			CategoryId: catID,
			CreatedAt:  formatTime(createdAt),
			Currency:   dbCurrency,
		},
	}), nil
}
