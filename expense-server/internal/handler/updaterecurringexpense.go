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

func (h *Handler) UpdateRecurringExpense(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateRecurringExpenseRequest],
) (*connect.Response[expensev1.UpdateRecurringExpenseResponse], error) {

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
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("next_run_date must be YYYY-MM-DD"))
	}

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

	currency := strings.TrimSpace(req.Msg.Currency)

	var query string
	var args []interface{}

	if currency != "" {
		query = `
			UPDATE recurring_expenses
			SET title = $1, amount = $2, category_id = $3, interval = $4, next_run_date = $5, is_active = $6, currency = $7
			WHERE id = $8 AND user_id = $9
			RETURNING id, user_id, title, amount, category_id, interval, next_run_date, last_run_date, is_active, created_at, currency`
		args = []interface{}{req.Msg.Title, req.Msg.Amount, dbCategoryID, interval, nextRunDate, req.Msg.IsActive, currency, req.Msg.Id, userID}
	} else {
		query = `
			UPDATE recurring_expenses
			SET title = $1, amount = $2, category_id = $3, interval = $4, next_run_date = $5, is_active = $6
			WHERE id = $7 AND user_id = $8
			RETURNING id, user_id, title, amount, category_id, interval, next_run_date, last_run_date, is_active, created_at, currency`
		args = []interface{}{req.Msg.Title, req.Msg.Amount, dbCategoryID, interval, nextRunDate, req.Msg.IsActive, req.Msg.Id, userID}
	}

	var (
		r             expensev1.RecurringExpense
		resCategoryID sql.NullInt32
		resNextRun    time.Time
		lastRun       sql.NullTime
		createdAt     time.Time
	)

	err = h.DB.QueryRowContext(ctx, query, args...).
		Scan(&r.Id, &r.UserId, &r.Title, &r.Amount, &resCategoryID, &r.Interval, &resNextRun, &lastRun, &r.IsActive, &createdAt, &r.Currency)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("recurring expense not found"))
		}
		log.Printf("ERROR updating recurring expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	r.CategoryId = resCategoryID.Int32
	r.Category = categoryName
	r.NextRunDate = resNextRun.Format("2006-01-02")
	if lastRun.Valid {
		r.LastRunDate = lastRun.Time.Format("2006-01-02")
	} else {
		r.LastRunDate = ""
	}
	r.CreatedAt = formatTime(createdAt)

	return connect.NewResponse(&expensev1.UpdateRecurringExpenseResponse{
		Message:          "recurring expense updated successfully",
		RecurringExpense: &r,
	}), nil
}
