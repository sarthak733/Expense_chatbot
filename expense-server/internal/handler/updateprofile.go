package handler

import (
	"context"
	"fmt"
	"log"
	"strings"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) UpdateProfile(
	ctx context.Context,
	req *connect.Request[expensev1.UpdateProfileRequest],
) (*connect.Response[expensev1.UpdateProfileResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	updater := h.EntClient.User.UpdateOneID(userID)

	if req.Msg.Email != nil {
		email := strings.TrimSpace(req.Msg.GetEmail())
		if email == "" {
			updater.ClearEmail()
		} else {
			updater.SetEmail(email)
		}
	}
	if req.Msg.FirstName != nil {
		fName := strings.TrimSpace(req.Msg.GetFirstName())
		if fName == "" {
			updater.ClearFirstName()
		} else {
			updater.SetFirstName(fName)
		}
	}
	if req.Msg.LastName != nil {
		lName := strings.TrimSpace(req.Msg.GetLastName())
		if lName == "" {
			updater.ClearLastName()
		} else {
			updater.SetLastName(lName)
		}
	}
	if req.Msg.Currency != nil {
		currency := strings.TrimSpace(req.Msg.GetCurrency())
		if currency == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("currency cannot be empty"))
		}
		updater.SetCurrency(currency)
	}
	if req.Msg.Theme != nil {
		theme := strings.ToLower(strings.TrimSpace(req.Msg.GetTheme()))
		if theme != "light" && theme != "dark" && theme != "system" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("theme must be light, dark, or system"))
		}
		updater.SetTheme(theme)
	}
	if req.Msg.WeeklyStart != nil {
		wStart := strings.ToLower(strings.TrimSpace(req.Msg.GetWeeklyStart()))
		if wStart != "monday" && wStart != "sunday" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("weekly_start must be monday or sunday"))
		}
		updater.SetWeeklyStart(wStart)
	}
	if req.Msg.MonthlyStartDay != nil {
		mStart := req.Msg.GetMonthlyStartDay()
		if mStart < 1 || mStart > 31 {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("monthly_start_day must be between 1 and 31"))
		}
		updater.SetMonthlyStartDay(int(mStart))
	}
	if req.Msg.BudgetAlertThreshold != nil {
		threshold := req.Msg.GetBudgetAlertThreshold()
		if threshold < 0 || threshold > 100 {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("budget_alert_threshold must be between 0 and 100"))
		}
		updater.SetBudgetAlertThreshold(threshold)
	}

	u, err := updater.Save(ctx)
	if err != nil {
		log.Printf("ERROR updating user profile: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update profile"))
	}

	return connect.NewResponse(&expensev1.UpdateProfileResponse{
		Message: "Profile updated successfully",
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
