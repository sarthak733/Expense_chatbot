package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
	"expense-server/internal/nlp"
)

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
