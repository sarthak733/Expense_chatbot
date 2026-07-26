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

func (h *Handler) ListRecurringExpenses(
	ctx context.Context,
	_ *connect.Request[expensev1.ListRecurringExpensesRequest],
) (*connect.Response[expensev1.ListRecurringExpensesResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT r.id, r.user_id, r.title, r.amount, r.category_id, COALESCE(c.name, 'Others'), r.interval, r.next_run_date, r.last_run_date, r.is_active, r.created_at, r.currency
		FROM recurring_expenses r
		LEFT JOIN categories c ON r.category_id = c.id
		WHERE r.user_id = $1
		ORDER BY r.created_at DESC`

	rows, err := h.DB.QueryContext(ctx, query, userID)
	if err != nil {
		log.Printf("ERROR listing recurring expenses: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	defer rows.Close()

	var list []*expensev1.RecurringExpense
	for rows.Next() {
		var (
			r           expensev1.RecurringExpense
			catID       sql.NullInt32
			nextRun     time.Time
			lastRun     sql.NullTime
			createdAt   time.Time
		)

		if err := rows.Scan(&r.Id, &r.UserId, &r.Title, &r.Amount, &catID, &r.Category, &r.Interval, &nextRun, &lastRun, &r.IsActive, &createdAt, &r.Currency); err != nil {
			log.Printf("ERROR scanning recurring expense: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}

		r.CategoryId = catID.Int32
		r.NextRunDate = nextRun.Format("2006-01-02")
		if lastRun.Valid {
			r.LastRunDate = lastRun.Time.Format("2006-01-02")
		} else {
			r.LastRunDate = ""
		}
		r.CreatedAt = formatTime(createdAt)

		list = append(list, &r)
	}

	if list == nil {
		list = []*expensev1.RecurringExpense{}
	}

	return connect.NewResponse(&expensev1.ListRecurringExpensesResponse{
		RecurringExpenses: list,
	}), nil
}
