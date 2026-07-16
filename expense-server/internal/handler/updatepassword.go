package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) UpdatePassword(
	ctx context.Context,
	req *connect.Request[expensev1.UpdatePasswordRequest],
) (*connect.Response[expensev1.UpdatePasswordResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	if req.Msg.OldPassword == "" || req.Msg.NewPassword == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("old_password and new_password are required"))
	}

	u, err := h.EntClient.User.Get(ctx, userID)
	if err != nil {
		log.Printf("ERROR getting user for password update: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Msg.OldPassword)); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid old password"))
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password"))
	}

	_, err = h.EntClient.User.UpdateOneID(userID).SetPasswordHash(string(newHash)).Save(ctx)
	if err != nil {
		log.Printf("ERROR saving new password hash: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save new password"))
	}

	return connect.NewResponse(&expensev1.UpdatePasswordResponse{
		Message: "Password updated successfully",
	}), nil
}
