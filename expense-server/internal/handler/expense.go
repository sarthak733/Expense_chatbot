package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	expensev1 "expense-server/gen/expense/v1"
	"expense-server/gen/expense/v1/expensev1connect"
	"expense-server/internal/exporter"
	"expense-server/internal/middleware"
	"expense-server/internal/nlp"
)

// Compile-time proof that Handler satisfies the generated service interface.
var _ expensev1connect.ExpenseServiceHandler = (*Handler)(nil)

// formatTime converts a DB time.Time to the RFC 3339 string used in the proto.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// HELPERS FOR CATEGORY RESOLUTION
// ---------------------------------------------------------------------------

// resolveCategory retrieves or creates a category based on the input name/ID.
func (h *Handler) resolveCategory(ctx context.Context, userID int, categoryID int32, categoryName string) (int32, string, error) {
	categoryName = strings.TrimSpace(categoryName)

	// Case 1: category_id is explicitly provided and valid
	if categoryID > 0 {
		var name string
		// Ensure the category exists and belongs to the user or is a system default (user_id IS NULL)
		query := `SELECT id, name FROM categories WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`
		err := h.DB.QueryRowContext(ctx, query, categoryID, userID).Scan(&categoryID, &name)
		if err == nil {
			return categoryID, name, nil
		}
		if err != sql.ErrNoRows {
			return 0, "", err
		}
	}

	// Case 2: category_id was not provided or not found, but categoryName is provided
	if categoryName != "" {
		var id int32
		var name string
		// Find exact case-insensitive match (system defaults or user's custom category)
		query := `SELECT id, name FROM categories WHERE LOWER(name) = LOWER($1) AND (user_id = $2 OR user_id IS NULL) LIMIT 1`
		err := h.DB.QueryRowContext(ctx, query, categoryName, userID).Scan(&id, &name)
		if err == nil {
			return id, name, nil
		}
		if err != sql.ErrNoRows {
			return 0, "", err
		}

		// If categoryName doesn't exist, create it as a custom category for the user
		insertQuery := `INSERT INTO categories (user_id, name, color) VALUES ($1, $2, '#808080') RETURNING id, name`
		err = h.DB.QueryRowContext(ctx, insertQuery, userID, categoryName).Scan(&id, &name)
		if err == nil {
			return id, name, nil
		}
		return 0, "", err
	}

	// Case 3: Neither provided, fallback to "Others" default category
	var id int32
	var name string
	err := h.DB.QueryRowContext(ctx, "SELECT id, name FROM categories WHERE name = 'Others' AND user_id IS NULL").Scan(&id, &name)
	if err != nil {
		// Fallback absolute backup if DB is in weird state
		return 0, "Others", nil
	}
	return id, name, nil
}

// ---------------------------------------------------------------------------
// CREATE EXPENSE
// ---------------------------------------------------------------------------

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
	catID, catName, err := h.resolveCategory(ctx, userID, req.Msg.CategoryId, req.Msg.Category)
	if err != nil {
		log.Printf("ERROR resolving category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	query := `
		INSERT INTO expenses (user_id, title, amount, category, category_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	var (
		id        int
		createdAt time.Time
	)
	err = h.DB.QueryRowContext(ctx, query, userID, req.Msg.Title, req.Msg.Amount, catName, catID).
		Scan(&id, &createdAt)
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
		},
	}), nil
}

// ---------------------------------------------------------------------------
// GET EXPENSE
// ---------------------------------------------------------------------------

func (h *Handler) GetExpense(
	ctx context.Context,
	req *connect.Request[expensev1.GetExpenseRequest],
) (*connect.Response[expensev1.GetExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT id, user_id, title, amount, COALESCE(category, ''), COALESCE(category_id, 0), created_at
		FROM expenses
		WHERE id = $1 AND user_id = $2`

	var (
		exp       expensev1.Expense
		createdAt time.Time
	)
	err := h.DB.QueryRowContext(ctx, query, req.Msg.Id, userID).
		Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("expense not found"))
		}
		log.Printf("ERROR fetching expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	exp.CreatedAt = formatTime(createdAt)

	return connect.NewResponse(&expensev1.GetExpenseResponse{
		Message: "expense retrieved successfully",
		Expense: &exp,
	}), nil
}

// ---------------------------------------------------------------------------
// LIST EXPENSES
// ---------------------------------------------------------------------------

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

	query := `SELECT id, user_id, title, amount, COALESCE(category, ''), COALESCE(category_id, 0), created_at FROM expenses WHERE user_id = $1`
	args := []interface{}{userID}
	argIdx := 2

	if req.Msg.StartDate != "" {
		if _, err := time.Parse(time.RFC3339, req.Msg.StartDate); err != nil {
			if _, err := time.Parse("2006-01-02", req.Msg.StartDate); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("start_date must be RFC3339 or YYYY-MM-DD"))
			}
		}
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, req.Msg.StartDate)
		countArgs = append(countArgs, req.Msg.StartDate)
		argIdx++
	}

	if req.Msg.EndDate != "" {
		if _, err := time.Parse(time.RFC3339, req.Msg.EndDate); err != nil {
			if _, err := time.Parse("2006-01-02", req.Msg.EndDate); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end_date must be RFC3339 or YYYY-MM-DD"))
			}
		}
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, req.Msg.EndDate)
		countArgs = append(countArgs, req.Msg.EndDate)
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
		if err := rows.Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt); err != nil {
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

// ---------------------------------------------------------------------------
// UPDATE EXPENSE
// ---------------------------------------------------------------------------

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

	catID, catName, err := h.resolveCategory(ctx, userID, req.Msg.CategoryId, req.Msg.Category)
	if err != nil {
		log.Printf("ERROR resolving category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	query := `
		UPDATE expenses
		SET title = $1, amount = $2, category = $3, category_id = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, title, amount, category, category_id, created_at`

	var (
		exp       expensev1.Expense
		createdAt time.Time
	)
	err = h.DB.QueryRowContext(ctx, query,
		req.Msg.Title, req.Msg.Amount, catName, catID, req.Msg.Id, userID,
	).Scan(&exp.Id, &exp.UserId, &exp.Title, &exp.Amount, &exp.Category, &exp.CategoryId, &createdAt)

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

// ---------------------------------------------------------------------------
// DELETE EXPENSE
// ---------------------------------------------------------------------------

func (h *Handler) DeleteExpense(
	ctx context.Context,
	req *connect.Request[expensev1.DeleteExpenseRequest],
) (*connect.Response[expensev1.DeleteExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `DELETE FROM expenses WHERE id = $1 AND user_id = $2`

	result, err := h.DB.ExecContext(ctx, query, req.Msg.Id, userID)
	if err != nil {
		log.Printf("ERROR deleting expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR checking rows affected: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("expense not found"))
	}

	return connect.NewResponse(&expensev1.DeleteExpenseResponse{
		Message: "expense deleted successfully",
	}), nil
}

// ---------------------------------------------------------------------------
// CATEGORIES CRUD
// ---------------------------------------------------------------------------

func (h *Handler) CreateCategory(
	ctx context.Context,
	req *connect.Request[expensev1.CreateCategoryRequest],
) (*connect.Response[expensev1.CreateCategoryResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("category name is required"))
	}

	color := req.Msg.Color
	if color == "" {
		color = "#808080"
	}

	// Insert custom category
	query := `
		INSERT INTO categories (user_id, name, color)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err := h.DB.QueryRowContext(ctx, query, userID, name, color).Scan(&id, &createdAt)
	if err != nil {
		// Category might already exist
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("category with this name already exists"))
	}

	return connect.NewResponse(&expensev1.CreateCategoryResponse{
		Message: "category created successfully",
		Category: &expensev1.Category{
			Id:        id,
			UserId:    int32(userID),
			Name:      name,
			Color:     color,
			CreatedAt: formatTime(createdAt),
		},
	}), nil
}

func (h *Handler) ListCategories(
	ctx context.Context,
	_ *connect.Request[expensev1.ListCategoriesRequest],
) (*connect.Response[expensev1.ListCategoriesResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Retrieve user-specific categories + system-wide default categories (user_id IS NULL)
	query := `
		SELECT id, COALESCE(user_id, 0), name, COALESCE(color, ''), created_at
		FROM categories
		WHERE user_id = $1 OR user_id IS NULL
		ORDER BY user_id NULLS FIRST, name ASC`

	rows, err := h.DB.QueryContext(ctx, query, userID)
	if err != nil {
		log.Printf("ERROR listing categories: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	defer rows.Close()

	var categories []*expensev1.Category
	for rows.Next() {
		var (
			cat       expensev1.Category
			createdAt time.Time
		)
		if err := rows.Scan(&cat.Id, &cat.UserId, &cat.Name, &cat.Color, &createdAt); err != nil {
			log.Printf("ERROR scanning category row: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}
		cat.CreatedAt = formatTime(createdAt)
		categories = append(categories, &cat)
	}

	if categories == nil {
		categories = []*expensev1.Category{}
	}

	return connect.NewResponse(&expensev1.ListCategoriesResponse{
		Categories: categories,
	}), nil
}

func (h *Handler) UpdateCategory(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateCategoryRequest],
) (*connect.Response[expensev1.UpdateCategoryResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("category name is required"))
	}

	color := req.Msg.Color
	if color == "" {
		color = "#808080"
	}

	// Update only custom categories (user_id = userID). System categories cannot be updated by users.
	query := `
		UPDATE categories
		SET name = $1, color = $2
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, name, color, created_at`

	var (
		cat       expensev1.Category
		createdAt time.Time
	)
	err := h.DB.QueryRowContext(ctx, query, name, color, req.Msg.Id, userID).
		Scan(&cat.Id, &cat.UserId, &cat.Name, &cat.Color, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("category not found or cannot be modified"))
		}
		log.Printf("ERROR updating category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}
	cat.CreatedAt = formatTime(createdAt)

	// Also update all expenses where category_id is this category, to keep the string 'category' field in sync!
	_, _ = h.DB.ExecContext(ctx, "UPDATE expenses SET category = $1 WHERE category_id = $2 AND user_id = $3", name, cat.Id, userID)

	return connect.NewResponse(&expensev1.UpdateCategoryResponse{
		Message:  "category updated successfully",
		Category: &cat,
	}), nil
}

func (h *Handler) DeleteCategory(
	ctx context.Context,
	req *connect.Request[expensev1.DeleteCategoryRequest],
) (*connect.Response[expensev1.DeleteCategoryResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Users can only delete their custom categories
	query := `DELETE FROM categories WHERE id = $1 AND user_id = $2`
	result, err := h.DB.ExecContext(ctx, query, req.Msg.Id, userID)
	if err != nil {
		log.Printf("ERROR deleting category: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR checking rows affected: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("category not found or cannot be deleted"))
	}

	return connect.NewResponse(&expensev1.DeleteCategoryResponse{
		Message: "category deleted successfully",
	}), nil
}

// ---------------------------------------------------------------------------
// BUDGETS CRUD & STATUS TRACKING
// ---------------------------------------------------------------------------

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

	query := `
		INSERT INTO budgets (user_id, category_id, amount, period, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err = h.DB.QueryRowContext(ctx, query, userID, dbCategoryID, req.Msg.Amount, period, startDate, endDate).
		Scan(&id, &createdAt)
	if err != nil {
		log.Printf("ERROR inserting budget: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	// Status tracking calculation (spent so far)
	var spent float64
	var totalSpentQuery string
	if dbCategoryID.Valid {
		totalSpentQuery = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at <= $4`
		_ = h.DB.QueryRowContext(ctx, totalSpentQuery, userID, dbCategoryID.Int32, startDate, endDate).Scan(&spent)
	} else {
		totalSpentQuery = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
		_ = h.DB.QueryRowContext(ctx, totalSpentQuery, userID, startDate, endDate).Scan(&spent)
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
		},
	}), nil
}

func (h *Handler) ListBudgets(
	ctx context.Context,
	_ *connect.Request[expensev1.ListBudgetsRequest],
) (*connect.Response[expensev1.ListBudgetsResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT id, user_id, category_id, amount, period, start_date, end_date, created_at
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
		)

		if err := rows.Scan(&b.Id, &b.UserId, &catID, &b.Amount, &b.Period, &startDate, &endDate, &createdAt); err != nil {
			log.Printf("ERROR scanning budget: %v", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
		}

		b.CategoryId = catID.Int32
		b.StartDate = startDate.Format("2006-01-02")
		b.EndDate = endDate.Format("2006-01-02")
		b.CreatedAt = formatTime(createdAt)

		// Calculate spent in range
		var spent float64
		if catID.Valid {
			// Query with category filter
			spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at < $4`
			// Exclude the day after endDate to capture exact timestamps
			nextDay := endDate.AddDate(0, 0, 1)
			_ = h.DB.QueryRowContext(ctx, spentQuery, userID, catID.Int32, startDate, nextDay).Scan(&spent)
		} else {
			// Overall budget
			spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`
			nextDay := endDate.AddDate(0, 0, 1)
			_ = h.DB.QueryRowContext(ctx, spentQuery, userID, startDate, nextDay).Scan(&spent)
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

	query := `
		UPDATE budgets
		SET category_id = $1, amount = $2, start_date = $3, end_date = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, category_id, amount, period, start_date, end_date, created_at`

	var (
		b             expensev1.Budget
		resCategoryID sql.NullInt32
		resStartDate  time.Time
		resEndDate    time.Time
		createdAt     time.Time
	)

	err = h.DB.QueryRowContext(ctx, query, dbCategoryID, req.Msg.Amount, startDate, endDate, req.Msg.Id, userID).
		Scan(&b.Id, &b.UserId, &resCategoryID, &b.Amount, &b.Period, &resStartDate, &resEndDate, &createdAt)

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

	// Calculate spent so far
	var spent float64
	if resCategoryID.Valid {
		spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at < $4`
		nextDay := resEndDate.AddDate(0, 0, 1)
		_ = h.DB.QueryRowContext(ctx, spentQuery, userID, resCategoryID.Int32, resStartDate, nextDay).Scan(&spent)
	} else {
		spentQuery := `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`
		nextDay := resEndDate.AddDate(0, 0, 1)
		_ = h.DB.QueryRowContext(ctx, spentQuery, userID, resStartDate, nextDay).Scan(&spent)
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

func (h *Handler) DeleteBudget(
	ctx context.Context,
	req *connect.Request[expensev1.DeleteBudgetRequest],
) (*connect.Response[expensev1.DeleteBudgetResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `DELETE FROM budgets WHERE id = $1 AND user_id = $2`
	result, err := h.DB.ExecContext(ctx, query, req.Msg.Id, userID)
	if err != nil {
		log.Printf("ERROR deleting budget: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR checking rows affected: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("budget not found"))
	}

	return connect.NewResponse(&expensev1.DeleteBudgetResponse{
		Message: "budget deleted successfully",
	}), nil
}

// ---------------------------------------------------------------------------
// RECURRING EXPENSES CRUD
// ---------------------------------------------------------------------------

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

	query := `
		INSERT INTO recurring_expenses (user_id, title, amount, category_id, interval, next_run_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err = h.DB.QueryRowContext(ctx, query, userID, req.Msg.Title, req.Msg.Amount, dbCategoryID, interval, nextRunDate).
		Scan(&id, &createdAt)
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
		},
	}), nil
}

func (h *Handler) ListRecurringExpenses(
	ctx context.Context,
	_ *connect.Request[expensev1.ListRecurringExpensesRequest],
) (*connect.Response[expensev1.ListRecurringExpensesResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `
		SELECT r.id, r.user_id, r.title, r.amount, r.category_id, COALESCE(c.name, 'Others'), r.interval, r.next_run_date, r.last_run_date, r.is_active, r.created_at
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

		if err := rows.Scan(&r.Id, &r.UserId, &r.Title, &r.Amount, &catID, &r.Category, &r.Interval, &nextRun, &lastRun, &r.IsActive, &createdAt); err != nil {
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

	query := `
		UPDATE recurring_expenses
		SET title = $1, amount = $2, category_id = $3, interval = $4, next_run_date = $5, is_active = $6
		WHERE id = $7 AND user_id = $8
		RETURNING id, user_id, title, amount, category_id, interval, next_run_date, last_run_date, is_active, created_at`

	var (
		r             expensev1.RecurringExpense
		resCategoryID sql.NullInt32
		resNextRun    time.Time
		lastRun       sql.NullTime
		createdAt     time.Time
	)

	err = h.DB.QueryRowContext(ctx, query, req.Msg.Title, req.Msg.Amount, dbCategoryID, interval, nextRunDate, req.Msg.IsActive, req.Msg.Id, userID).
		Scan(&r.Id, &r.UserId, &r.Title, &r.Amount, &resCategoryID, &r.Interval, &resNextRun, &lastRun, &r.IsActive, &createdAt)

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

func (h *Handler) DeleteRecurringExpense(
	ctx context.Context,
	req *connect.Request[expensev1.DeleteRecurringExpenseRequest],
) (*connect.Response[expensev1.DeleteRecurringExpenseResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	query := `DELETE FROM recurring_expenses WHERE id = $1 AND user_id = $2`
	result, err := h.DB.ExecContext(ctx, query, req.Msg.Id, userID)
	if err != nil {
		log.Printf("ERROR deleting recurring expense: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR checking rows affected: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("recurring expense not found"))
	}

	return connect.NewResponse(&expensev1.DeleteRecurringExpenseResponse{
		Message: "recurring expense deleted successfully",
	}), nil
}

// ---------------------------------------------------------------------------
// EXPORTS
// ---------------------------------------------------------------------------

func (h *Handler) ExportExpensesCSV(
	ctx context.Context,
	req *connect.Request[expensev1.ExportExpensesRequest],
) (*connect.Response[expensev1.ExportExpensesCSVResponse], error) {

	_, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Reuse ListExpenses logic to retrieve filtered list of expenses
	listReq := &connect.Request[expensev1.ListExpensesRequest]{
		Msg: &expensev1.ListExpensesRequest{
			StartDate: req.Msg.StartDate,
			EndDate:   req.Msg.EndDate,
			Limit:     100000, // very large limit to export everything
		},
	}
	res, err := h.ListExpenses(ctx, listReq)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write CSV Header
	_ = writer.Write([]string{"ID", "Date", "Description", "Category", "Amount"})

	for _, exp := range res.Msg.Expenses {
		_ = writer.Write([]string{
			strconv.Itoa(int(exp.Id)),
			exp.CreatedAt,
			exp.Title,
			exp.Category,
			fmt.Sprintf("%.2f", exp.Amount),
		})
	}
	writer.Flush()

	return connect.NewResponse(&expensev1.ExportExpensesCSVResponse{
		CsvData: buf.Bytes(),
	}), nil
}

func (h *Handler) ExportExpensesPDF(
	ctx context.Context,
	req *connect.Request[expensev1.ExportExpensesRequest],
) (*connect.Response[expensev1.ExportExpensesPDFResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Get username for PDF branding
	var username string
	_ = h.DB.QueryRowContext(ctx, "SELECT username FROM users WHERE id = $1", userID).Scan(&username)

	listReq := &connect.Request[expensev1.ListExpensesRequest]{
		Msg: &expensev1.ListExpensesRequest{
			StartDate: req.Msg.StartDate,
			EndDate:   req.Msg.EndDate,
			Limit:     100000,
		},
	}
	res, err := h.ListExpenses(ctx, listReq)
	if err != nil {
		return nil, err
	}

	pdfData, err := exporter.GenerateExpensesPDF(res.Msg.Expenses, username)
	if err != nil {
		log.Printf("ERROR generating PDF: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate PDF"))
	}

	return connect.NewResponse(&expensev1.ExportExpensesPDFResponse{
		PdfData: pdfData,
	}), nil
}

// ---------------------------------------------------------------------------
// NLP PARSING & QUICK ADD
// ---------------------------------------------------------------------------

func (h *Handler) ParseExpenseText(
	ctx context.Context,
	req *connect.Request[expensev1.ParseExpenseTextRequest],
) (*connect.Response[expensev1.ParseExpenseTextResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	parsed := nlp.ParseText(req.Msg.Text)

	// Resolve the parsed category to an ID (without committing it yet)
	var catID int32
	var catName string = parsed.Category
	query := `SELECT id, name FROM categories WHERE LOWER(name) = LOWER($1) AND (user_id = $2 OR user_id IS NULL) LIMIT 1`
	err := h.DB.QueryRowContext(ctx, query, parsed.Category, userID).Scan(&catID, &catName)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("ERROR looking up category in NLP: %v", err)
	}

	return connect.NewResponse(&expensev1.ParseExpenseTextResponse{
		Title:      parsed.Title,
		Amount:     parsed.Amount,
		Category:   catName,
		CategoryId: catID,
		Date:       formatTime(parsed.Date),
	}), nil
}

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
	catID, catName, err := h.resolveCategory(ctx, userID, 0, parsed.Category)
	if err != nil {
		log.Printf("ERROR resolving category in QuickAdd: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	query := `
		INSERT INTO expenses (user_id, title, amount, category, category_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	var id int32
	var createdAt time.Time
	err = h.DB.QueryRowContext(ctx, query, userID, parsed.Title, parsed.Amount, catName, catID, parsed.Date).
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
		},
	}), nil
}

// ---------------------------------------------------------------------------
// MONTHLY COMPARISON INSIGHTS
// ---------------------------------------------------------------------------

func (h *Handler) GetMonthlyInsights(
	ctx context.Context,
	_ *connect.Request[expensev1.GetMonthlyInsightsRequest],
) (*connect.Response[expensev1.GetMonthlyInsightsResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	currentMonthEndExcl := currentMonthStart.AddDate(0, 1, 0)

	prevMonthStart := currentMonthStart.AddDate(0, -1, 0)
	prevMonthEndExcl := currentMonthStart

	// 1. Get totals
	var currentTotal, prevTotal float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3", userID, currentMonthStart, currentMonthEndExcl).Scan(&currentTotal)
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3", userID, prevMonthStart, prevMonthEndExcl).Scan(&prevTotal)

	totalDiffPercent := 0.0
	if prevTotal > 0 {
		totalDiffPercent = ((currentTotal - prevTotal) / prevTotal) * 100.0
	}

	// 2. Get category breakdown
	type catSpent struct {
		Name  string
		Spent float64
	}

	// Fetch current month categories
	currentCatQuery := `
		SELECT COALESCE(category, 'Others'), SUM(amount)
		FROM expenses
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY COALESCE(category, 'Others')`
	rowsCur, err := h.DB.QueryContext(ctx, currentCatQuery, userID, currentMonthStart, currentMonthEndExcl)
	if err != nil {
		log.Printf("ERROR fetching current month category spent: %v", err)
	}
	
	currentCats := make(map[string]float64)
	if err == nil {
		defer rowsCur.Close()
		for rowsCur.Next() {
			var name string
			var spent float64
			if err := rowsCur.Scan(&name, &spent); err == nil {
				currentCats[name] = spent
			}
		}
	}

	// Fetch prev month categories
	rowsPrev, err := h.DB.QueryContext(ctx, currentCatQuery, userID, prevMonthStart, prevMonthEndExcl)
	prevCats := make(map[string]float64)
	if err == nil {
		defer rowsPrev.Close()
		for rowsPrev.Next() {
			var name string
			var spent float64
			if err := rowsPrev.Scan(&name, &spent); err == nil {
				prevCats[name] = spent
			}
		}
	}

	// Compute differences for all unique categories seen across both months
	allCats := make(map[string]bool)
	for k := range currentCats {
		allCats[k] = true
	}
	for k := range prevCats {
		allCats[k] = true
	}

	var insights []*expensev1.CategoryInsight
	for cat := range allCats {
		curS := currentCats[cat]
		preS := prevCats[cat]
		diffP := 0.0
		if preS > 0 {
			diffP = ((curS - preS) / preS) * 100.0
		}
		insights = append(insights, &expensev1.CategoryInsight{
			Category:       cat,
			CurrentSpent:   curS,
			PrevSpent:      preS,
			DiffPercentage: diffP,
		})
	}

	// 3. Generate narrative insights
	var generalInsights []string
	if currentTotal > prevTotal && prevTotal > 0 {
		generalInsights = append(generalInsights, fmt.Sprintf("Your spending increased by %.1f%% compared to last month. Take a close look at your category budgets.", totalDiffPercent))
	} else if currentTotal < prevTotal && prevTotal > 0 {
		savings := prevTotal - currentTotal
		generalInsights = append(generalInsights, fmt.Sprintf("Great job! You spent $%.2f less than last month (a savings of %.1f%%).", savings, -totalDiffPercent))
	} else if prevTotal == 0 {
		generalInsights = append(generalInsights, "This is your second month tracking expenses! Compare your spendings as the month progresses.")
	}

	// Check specific categories with high spikes
	for _, ins := range insights {
		if ins.DiffPercentage > 25.0 && ins.CurrentSpent > 50.0 {
			generalInsights = append(generalInsights, fmt.Sprintf("Alert: Spending on '%s' has spiked by %.1f%% compared to last month.", ins.Category, ins.DiffPercentage))
		}
	}

	// Check overall budget compliance
	var totalBudgets, exceededBudgets int
	budgetsRows, err := h.DB.QueryContext(ctx, "SELECT id, category_id, amount, start_date, end_date FROM budgets WHERE user_id = $1", userID)
	if err == nil {
		defer budgetsRows.Close()
		for budgetsRows.Next() {
			var bid int
			var cid sql.NullInt32
			var bAmt float64
			var sDate, eDate time.Time
			if err := budgetsRows.Scan(&bid, &cid, &bAmt, &sDate, &eDate); err == nil {
				totalBudgets++
				var bSpent float64
				if cid.Valid {
					_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND category_id = $2 AND created_at >= $3 AND created_at < $4", userID, cid.Int32, sDate, eDate.AddDate(0, 0, 1)).Scan(&bSpent)
				} else {
					_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND created_at >= $2 AND created_at < $3", userID, sDate, eDate.AddDate(0, 0, 1)).Scan(&bSpent)
				}
				if bSpent > bAmt {
					exceededBudgets++
				}
			}
		}
	}
	if exceededBudgets > 0 {
		generalInsights = append(generalInsights, fmt.Sprintf("You have exceeded %d out of your %d active budgets.", exceededBudgets, totalBudgets))
	} else if totalBudgets > 0 && exceededBudgets == 0 {
		generalInsights = append(generalInsights, "Keep it up! You are currently within all of your registered budgets.")
	}

	return connect.NewResponse(&expensev1.GetMonthlyInsightsResponse{
		CurrentMonth:         now.Format("January 2006"),
		PrevMonth:            now.AddDate(0, -1, 0).Format("January 2006"),
		CurrentTotal:         currentTotal,
		PrevTotal:            prevTotal,
		TotalDiffPercentage: totalDiffPercent,
		CategoryInsights:     insights,
		GeneralInsights:      generalInsights,
	}), nil
}
