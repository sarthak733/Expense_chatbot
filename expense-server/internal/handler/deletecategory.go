package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

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
