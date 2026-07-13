package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"expense-server/internal/model"
	"expense-server/internal/response"
)

// CREATE
func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	// Define an anonymous struct to parse only the allowed input fields
	var input struct {
		UserID   int     `json:"user_id"`
		Title    string  `json:"title"`
		Amount   float64 `json:"amount"`
		Category string  `json:"category"`
	}

	// 1. Decode JSON payload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.UserID <= 0 || input.Title == "" || input.Amount <= 0 || input.Category == "" {
		response.Error(w, http.StatusUnprocessableEntity, "missing fields or invalid amount")
		return
	}

	// 2. Simple Validation
	if input.Title == "" || input.Amount <= 0 || input.Category == "" {
		response.Error(w, http.StatusUnprocessableEntity, "missing fields or invalid amount")
		return
	}

	// 3. Prepare the SQL query using RETURNING to get DB-generated fields
	query := `
		INSERT INTO expenses (user_id, title, amount, category)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	var exp model.Expense
	exp.UserID = input.UserID
	exp.Title = input.Title
	exp.Amount = input.Amount
	exp.Category = input.Category

	// Execute query and scan database-generated fields directly into our model
	err := h.DB.QueryRowContext(r.Context(), query, exp.UserID, exp.Title, exp.Amount, exp.Category).
		Scan(&exp.ID, &exp.CreatedAt)

	if err != nil {
		log.Printf("ERROR inserting expense: %v", err)
		// Keep error details internal (log them), send safe message to frontend
		http.Error(w, "internal database error", http.StatusInternalServerError)
		return
	}

	// 4. Return successful response with 201 Created
	response.JSON(w, http.StatusCreated, "expense created successfully", exp)
}

// READ
// ListExpenses fetches all expense records from the database
func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, user_id, title, amount, category, created_at FROM expenses ORDER BY created_at DESC`

	rows, err := h.DB.QueryContext(r.Context(), query)
	if err != nil {
		log.Printf("ERROR listing expenses: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}
	defer rows.Close()

	expenses := []model.Expense{}

	for rows.Next() {
		var exp model.Expense
		if err := rows.Scan(&exp.ID, &exp.UserID, &exp.Title, &exp.Amount, &exp.Category, &exp.CreatedAt); err != nil {
			log.Printf("ERROR scanning expense row: %v", err)
			response.Error(w, http.StatusInternalServerError, "internal database error")
			return
		}
		expenses = append(expenses, exp)
	}

	// Check for errors encountered during iteration
	if err = rows.Err(); err != nil {
		log.Printf("ERROR after row iteration: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}

	response.JSON(w, http.StatusOK, "expenses retrieved successfully", expenses)
}

// GetExpense fetches a single expense record using its path ID
func (h *Handler) GetExpense(w http.ResponseWriter, r *http.Request) {
	// PathValue reads wildcards from Go 1.22+ routing syntax
	id := r.PathValue("id")

	query := `SELECT id, user_id, title, amount, category, created_at FROM expenses WHERE id = $1`

	var exp model.Expense
	err := h.DB.QueryRowContext(r.Context(), query, id).
		Scan(&exp.ID, &exp.UserID, &exp.Title, &exp.Amount, &exp.Category, &exp.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, http.StatusNotFound, "expense not found")
			return
		}
		log.Printf("ERROR fetching individual expense: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}

	response.JSON(w, http.StatusOK, "expense retrieved successfully", exp)
}

//UPDATE

// UpdateExpense modifies an existing expense record by ID
func (h *Handler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Title    string  `json:"title"`
		Amount   float64 `json:"amount"`
		Category string  `json:"category"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Title == "" || input.Amount <= 0 || input.Category == "" {
		response.Error(w, http.StatusUnprocessableEntity, "missing fields or invalid amount")
		return
	}

	query := `
		UPDATE expenses 
		SET title = $1, amount = $2, category = $3 
		WHERE id = $4
		RETURNING id, title, amount, category, created_at`

	var exp model.Expense
	err := h.DB.QueryRowContext(r.Context(), query, input.Title, input.Amount, input.Category, id).
		Scan(&exp.ID, &exp.Title, &exp.Amount, &exp.Category, &exp.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, http.StatusNotFound, "expense not found")
			return
		}
		log.Printf("ERROR updating expense: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}

	response.JSON(w, http.StatusOK, "expense updated successfully", exp)
}

// DeleteExpense removes an expense record from the database by ID
func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	query := `DELETE FROM expenses WHERE id = $1`

	result, err := h.DB.ExecContext(r.Context(), query, id)
	if err != nil {
		log.Printf("ERROR deleting expense: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}

	// RowsAffected checks if the ID actually existed to be deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR checking rows affected: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal database error")
		return
	}

	if rowsAffected == 0 {
		response.Error(w, http.StatusNotFound, "expense not found")
		return
	}

	response.JSON(w, http.StatusOK, "expense deleted successfully", nil)
}
