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

func (h *Handler) ListBudgets(
	ctx context.Context,
	_ *connect.Request[expensev1.ListBudgetsRequest],
) (*connect.Response[expensev1.ListBudgetsResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT id, user_id, category_id, amount, period, start_date, end_date, created_at, currency
		FROM budgets
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := h.DB.QueryContext(ctx, query, userID)
	if err != nil {
		log.Printf("ERROR listing budgets: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	defer rows.Close()

	var budgets []*expensev1.Budget
	for rows.Next() {
		var (
			b          expensev1.Budget
			catID      sql.NullInt32
			startDate  time.Time
			endDate    time.Time
			createdAt  time.Time
			bCurrency  string
		)

		if err := rows.Scan(&b.Id, &b.UserId, &catID, &b.Amount, &b.Period, &startDate, &endDate, &createdAt, &bCurrency); err != nil {
			log.Printf("ERROR scanning budget: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}

		b.CategoryId = catID.Int32
		b.StartDate = startDate.Format("2006-01-02")
		b.EndDate = endDate.Format("2006-01-02")
		b.CreatedAt = formatTime(createdAt)
		b.Currency = bCurrency

		// Calculate spent in range — only include expenses with the same currency as this budget.
		var spent float64
		if catID.Valid {
			spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at < $4 AND currency = $5`
			nextDay := endDate.AddDate(0, 0, 1)
			_ = h.DB.QueryRowContext(ctx, spentQuery, userID, catID.Int32, startDate, nextDay, bCurrency).Scan(&spent)
		} else {
			spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND currency = $4`
			nextDay := endDate.AddDate(0, 0, 1)
			_ = h.DB.QueryRowContext(ctx, spentQuery, userID, startDate, nextDay, bCurrency).Scan(&spent)
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

		budgets = append(budgets, &b)
	}

	if budgets == nil {
		budgets = []*expensev1.Budget{}
	}

	return connect.NewResponse(&expensev1.ListBudgetsResponse{
		Budgets: budgets,
	}), nil
}
