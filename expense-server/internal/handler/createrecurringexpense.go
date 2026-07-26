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

func (h *Handler) CreateRecurringExpense(
	ctx context.Context,
	req *connect.Request[expensev1.CreateRecurringExpenseRequest],
) (*connect.Response[expensev1.CreateRecurringExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.Title == "" || req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title and amount (>0) are required"))
	}

	interval := strings.ToLower(req.Msg.Interval)
	if interval != "daily" && interval != "weekly" && interval != "monthly" && interval != "yearly" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interval must be daily, weekly, monthly, or yearly"))
	}

	nextRunDate, err := time.Parse("2006-01-02", req.Msg.NextRunDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("next_run_date must be in YYYY-MM-DD format"))
	}

	// Validate category_id
	var dbCategoryID sql.NullInt32
	var categoryName string = "Others"
	if req.Msg.CategoryId > 0 {
		err = h.DB.QueryRowContext(ctx, "SELECT name FROM categories WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)", req.Msg.CategoryId, userID).Scan(&categoryName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid category_id"))
		}
		dbCategoryID.Int32 = req.Msg.CategoryId
		dbCategoryID.Valid = true
	}

	// Read currency from request, fallback to user's profile currency
	currency := strings.TrimSpace(req.Msg.Currency)
	if currency == "" {
		currency, _ = getUserCurrency(ctx, h.DB, userID)
	}

	query := `
		INSERT INTO recurring_expenses (user_id, title, amount, category_id, interval, next_run_date, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, currency`

	var id int32
	var createdAt time.Time
	var dbCurrency string
	err = h.DB.QueryRowContext(ctx, query, userID, req.Msg.Title, req.Msg.Amount, dbCategoryID, interval, nextRunDate, currency).
		Scan(&id, &createdAt, &dbCurrency)
	if err != nil {
		log.Printf("ERROR inserting recurring expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	return connect.NewResponse(&expensev1.CreateRecurringExpenseResponse{
		Message: "recurring expense created successfully",
		RecurringExpense: &expensev1.RecurringExpense{
			Id:          id,
			UserId:      int32(userID),
			Title:       req.Msg.Title,
			Amount:      req.Msg.Amount,
			CategoryId:  req.Msg.CategoryId,
			Category:    categoryName,
			Interval:    interval,
			NextRunDate: req.Msg.NextRunDate,
			IsActive:    true,
			CreatedAt:   formatTime(createdAt),
			Currency:    dbCurrency,
		},
	}), nil
}
