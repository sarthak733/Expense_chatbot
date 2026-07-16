package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	"expense-server/ent"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) GetProfile(
	ctx context.Context,
	_ *connect.Request[expensev1.GetProfileRequest],
) (*connect.Response[expensev1.GetProfileResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	u, err := h.EntClient.User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
		}
		log.Printf("ERROR getting user profile: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal database error"))
	}

	return connect.NewResponse(&expensev1.GetProfileResponse{
		Profile: &expensev1.UserProfile{
			Id:                   int32(u.ID),
			Username:             u.Username,
			Email:                getStringValue(u.Email),
			FirstName:            getStringValue(u.FirstName),
			LastName:             getStringValue(u.LastName),
			Currency:             u.Currency,
			Theme:                u.Theme,
			WeeklyStart:          u.WeeklyStart,
			MonthlyStartDay:      int32(u.MonthlyStartDay),
			BudgetAlertThreshold: u.BudgetAlertThreshold,
			CreatedAt:            formatTime(u.CreatedAt),
		},
	}), nil
}

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
