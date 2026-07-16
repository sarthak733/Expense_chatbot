package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

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
