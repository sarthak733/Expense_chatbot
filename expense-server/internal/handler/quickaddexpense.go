package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
	"expense-server/internal/nlp"
)

func (h *Handler) QuickAddExpense(
	ctx context.Context,
	req *connect.Request[expensev1.QuickAddExpenseRequest],
) (*connect.Response[expensev1.QuickAddExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	parsed := nlp.ParseText(req.Msg.Text)
	if parsed.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("could not parse valid expense amount from input"))
	}

	// Resolve/Create the category
	catID, catName, err := h.resolveCategory(ctx, userID, 0, parsed.Category, parsed.Title)
	if err != nil {
		log.Printf("ERROR resolving category in QuickAdd: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	// Use currency detected from the input text; fallback to user's profile currency.
	currency := parsed.Currency
	if currency == "" {
		currency, _ = getUserCurrency(ctx, h.DB, userID)
	}

	query := `
		INSERT INTO expenses (user_id, title, amount, category, category_id, created_at, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err = h.DB.QueryRowContext(ctx, query, userID, parsed.Title, parsed.Amount, catName, catID, parsed.Date, currency).
		Scan(&id, &createdAt)
	if err != nil {
		log.Printf("ERROR quick-adding expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	return connect.NewResponse(&expensev1.QuickAddExpenseResponse{
		Message: "expense quick-added successfully",
		Expense: &expensev1.Expense{
			Id:         id,
			UserId:     int32(userID),
			Title:      parsed.Title,
			Amount:     parsed.Amount,
			Category:   catName,
			CategoryId: catID,
			CreatedAt:  formatTime(createdAt),
			Currency:   currency,
		},
	}), nil
}
