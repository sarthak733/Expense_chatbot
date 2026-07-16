package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

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
