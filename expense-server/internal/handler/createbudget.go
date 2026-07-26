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

func (h *Handler) CreateBudget(
	ctx context.Context,
	req *connect.Request[expensev1.CreateBudgetRequest],
) (*connect.Response[expensev1.CreateBudgetResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.Amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("budget amount must be greater than 0"))
	}

	period := strings.ToLower(req.Msg.Period)
	if period != "weekly" && period != "monthly" && period != "yearly" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("period must be weekly, monthly, or yearly"))
	}

	// Parse start and end dates
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

	// If category_id is > 0, make sure it is valid
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

	// Read currency from request, fallback to user's profile currency
	currency := strings.TrimSpace(req.Msg.Currency)
	if currency == "" {
		currency, _ = getUserCurrency(ctx, h.DB, userID)
	}

	query := `
		INSERT INTO budgets (user_id, category_id, amount, period, start_date, end_date, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, currency`

	var id int32
	var createdAt time.Time
	var dbCurrency string
	err = h.DB.QueryRowContext(ctx, query, userID, dbCategoryID, req.Msg.Amount, period, startDate, endDate, currency).
		Scan(&id, &createdAt, &dbCurrency)
	if err != nil {
		log.Printf("ERROR inserting budget: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	// Status tracking calculation (spent so far).
	// Only count expenses in the same currency as this budget to avoid cross-currency mixing.
	var spent float64
	if dbCategoryID.Valid {
		_ = h.DB.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at <= $4 AND currency = $5`,
			userID, dbCategoryID.Int32, startDate, endDate, currency).Scan(&spent)
	} else {
		_ = h.DB.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND currency = $4`,
			userID, startDate, endDate, currency).Scan(&spent)
	}

	remaining := req.Msg.Amount - spent
	percentage := (spent / req.Msg.Amount) * 100.0
	status := "green"
	if percentage >= 100.0 {
		status = "exceeded"
	} else if percentage >= 80.0 {
		status = "warning"
	}

	return connect.NewResponse(&expensev1.CreateBudgetResponse{
		Message: "budget created successfully",
		Budget: &expensev1.Budget{
			Id:         id,
			UserId:     int32(userID),
			CategoryId: req.Msg.CategoryId,
			Amount:     req.Msg.Amount,
			Period:     period,
			StartDate:  req.Msg.StartDate,
			EndDate:    req.Msg.EndDate,
			CreatedAt:  formatTime(createdAt),
			Spent:      spent,
			Remaining:  remaining,
			Percentage: percentage,
			Status:     status,
			Currency:   dbCurrency,
		},
	}), nil
}
