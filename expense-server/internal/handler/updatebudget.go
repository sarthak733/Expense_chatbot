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

func (h *Handler) UpdateBudget(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateBudgetRequest],
) (*connect.Response[expensev1.UpdateBudgetResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("budget amount must be greater than 0"))
	}

	startDate, err := time.Parse("2006-01-02", req.Msg.StartDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("start_date must be in YYYY-MM-DD format"))
	}
	endDate, err := time.Parse("2006-01-02", req.Msg.EndDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end_date must be in YYYY-MM-DD format"))
	}

	if endDate.Before(startDate) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end_date must be equal to or after start_date"))
	}

	var dbCategoryID sql.NullInt32
	if req.Msg.CategoryId > 0 {
		var exists bool
		err = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND (user_id = $2 OR user_id IS NULL))", req.Msg.CategoryId, userID).Scan(&exists)
		if err != nil || !exists {
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
			UPDATE budgets
			SET category_id = $1, amount = $2, start_date = $3, end_date = $4, currency = $5
			WHERE id = $6 AND user_id = $7
			RETURNING id, user_id, category_id, amount, period, start_date, end_date, created_at, currency`
		args = []interface{}{dbCategoryID, req.Msg.Amount, startDate, endDate, currency, req.Msg.Id, userID}
	} else {
		query = `
			UPDATE budgets
			SET category_id = $1, amount = $2, start_date = $3, end_date = $4
			WHERE id = $5 AND user_id = $6
			RETURNING id, user_id, category_id, amount, period, start_date, end_date, created_at, currency`
		args = []interface{}{dbCategoryID, req.Msg.Amount, startDate, endDate, req.Msg.Id, userID}
	}

	var (
		b             expensev1.Budget
		resCategoryID sql.NullInt32
		resStartDate  time.Time
		resEndDate    time.Time
		createdAt     time.Time
	)

	err = h.DB.QueryRowContext(ctx, query, args...).
		Scan(&b.Id, &b.UserId, &resCategoryID, &b.Amount, &b.Period, &resStartDate, &resEndDate, &createdAt, &b.Currency)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("budget not found"))
		}
		log.Printf("ERROR updating budget: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	b.CategoryId = resCategoryID.Int32
	b.StartDate = resStartDate.Format("2006-01-02")
	b.EndDate = resEndDate.Format("2006-01-02")
	b.CreatedAt = formatTime(createdAt)

	// Calculate spent so far, matching the budget's currency
	var spent float64
	if resCategoryID.Valid {
		spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at < $4 AND currency = $5`
		nextDay := resEndDate.AddDate(0, 0, 1)
		_ = h.DB.QueryRowContext(ctx, spentQuery, userID, resCategoryID.Int32, resStartDate, nextDay, b.Currency).Scan(&spent)
	} else {
		spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND currency = $4`
		nextDay := resEndDate.AddDate(0, 0, 1)
		_ = h.DB.QueryRowContext(ctx, spentQuery, userID, resStartDate, nextDay, b.Currency).Scan(&spent)
	}

	b.Spent = spent
	b.Remaining = b.Amount - spent
	if b.Amount > 0 {
		b.Percentage = (spent / b.Amount) * 100.0
	}
	
	b.Status = "green"
	if b.Percentage >= 100.0 {
		b.Status = "exceeded"
	} else if b.Percentage >= 80.0 {
		b.Status = "warning"
	}

	return connect.NewResponse(&expensev1.UpdateBudgetResponse{
		Message: "budget updated successfully",
		Budget:  &b,
	}), nil
}
