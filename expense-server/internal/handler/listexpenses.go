package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) ListExpenses(
	ctx context.Context,
	req *connect.Request[expensev1.ListExpensesRequest],
) (*connect.Response[expensev1.ListExpensesResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	countQuery := `SELECT COUNT(*) FROM expenses WHERE user_id = $1`
	countArgs := []interface{}{userID}

	query := `SELECT id, user_id, title, amount, COALESCE(category, ''), COALESCE(category_id, 0), created_at, currency FROM expenses WHERE user_id = $1`
	args := []interface{}{userID}
	argIdx := 2

	if req.Msg.StartDate != "" {
		var t time.Time
		var err error
		t, err = time.Parse(time.RFC3339, req.Msg.StartDate)
		if err != nil {
			t, err = time.ParseInLocation("2006-01-02", req.Msg.StartDate, time.Local)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("start_date must be RFC3339 or YYYY-MM-DD"))
			}
		}
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, t)
		countArgs = append(countArgs, t)
		argIdx++
	}

	if req.Msg.EndDate != "" {
		var t time.Time
		var err error
		t, err = time.Parse(time.RFC3339, req.Msg.EndDate)
		if err != nil {
			t, err = time.ParseInLocation("2006-01-02", req.Msg.EndDate, time.Local)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end_date must be RFC3339 or YYYY-MM-DD"))
			}
			t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
		}
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, t)
		countArgs = append(countArgs, t)
		argIdx++
	}

	var totalCount int32
	if err := h.DB.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		log.Printf("ERROR counting expenses: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	query += " ORDER BY created_at DESC"

	if req.Msg.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, req.Msg.Limit)
		argIdx++
	}
	if req.Msg.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, req.Msg.Offset)
		argIdx++
	}

	rows, err := h.DB.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("ERROR listing expenses: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	defer rows.Close()

	var expenses []*expensev1.Expense

	for rows.Next() {
		var (
			exp       expensev1.Expense
			createdAt time.Time
		)
		if err := rows.Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt, &exp.Currency); err != nil {
			log.Printf("ERROR scanning expense row: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}
		exp.CreatedAt = formatTime(createdAt)
		expenses = append(expenses, &exp)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR after row iteration: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	if expenses == nil {
		expenses = []*expensev1.Expense{}
	}

	return connect.NewResponse(&expensev1.ListExpensesResponse{
		Message:    "expenses retrieved successfully",
		Expenses:   expenses,
		TotalCount: totalCount,
	}), nil
}
