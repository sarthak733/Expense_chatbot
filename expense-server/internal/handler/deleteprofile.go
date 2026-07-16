package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	"expense-server/ent/budget"
	"expense-server/ent/category"
	"expense-server/ent/chatmessage"
	"expense-server/ent/expense"
	"expense-server/ent/recurringexpense"
	"expense-server/ent/session"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) DeleteProfile(
	ctx context.Context,
	_ *connect.Request[expensev1.DeleteProfileRequest],
) (*connect.Response[expensev1.DeleteProfileResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Run within a transaction to guarantee atomicity
	tx, err := h.EntClient.Tx(ctx)
	if err != nil {
		log.Printf("ERROR starting transaction for profile deletion: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to start transaction"))
	}
	defer tx.Rollback()

	// 1. Delete user sessions
	_, err = tx.Session.Delete().Where(session.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete user sessions: %w", err))
	}

	// 2. Delete chat messages
	_, err = tx.ChatMessage.Delete().Where(chatmessage.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete chat messages: %w", err))
	}

	// 3. Delete expenses
	_, err = tx.Expense.Delete().Where(expense.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete expenses: %w", err))
	}

	// 4. Delete budgets
	_, err = tx.Budget.Delete().Where(budget.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete budgets: %w", err))
	}

	// 5. Delete recurring expenses
	_, err = tx.RecurringExpense.Delete().Where(recurringexpense.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete recurring expenses: %w", err))
	}

	// 6. Delete custom categories (user_id = userID)
	_, err = tx.Category.Delete().Where(category.UserID(userID)).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete custom categories: %w", err))
	}

	// 7. Delete the user
	err = tx.User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete user record: %w", err))
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR committing transaction for profile deletion: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction"))
	}

	return connect.NewResponse(&expensev1.DeleteProfileResponse{
		Message: "User profile and all associated data deleted successfully",
	}), nil
}
