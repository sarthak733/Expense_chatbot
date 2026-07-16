package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

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
