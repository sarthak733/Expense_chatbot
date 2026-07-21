package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

func (h *Handler) ExportExpensesCSV(
	ctx context.Context,
	req *connect.Request[expensev1.ExportExpensesRequest],
) (*connect.Response[expensev1.ExportExpensesCSVResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Fetch user's preferred currency for display in the export.
	currency, _ := getUserCurrency(ctx, h.DB, userID)
	sym := currencySymbol(currency)

	// Reuse ListExpenses logic to retrieve filtered list of expenses
	listReq := &connect.Request[expensev1.ListExpensesRequest]{
		Msg: &expensev1.ListExpensesRequest{
			StartDate: req.Msg.StartDate,
			EndDate:   req.Msg.EndDate,
			Limit:     100000, // very large limit to export everything
		},
	}
	res, err := h.ListExpenses(ctx, listReq)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write CSV Header
	_ = writer.Write([]string{"ID", "Date", "Description", "Category", "Currency", "Amount"})

	for _, exp := range res.Msg.Expenses {
		_ = writer.Write([]string{
			strconv.Itoa(int(exp.Id)),
			exp.CreatedAt,
			exp.Title,
			exp.Category,
			currency,
			fmt.Sprintf("%s%.2f", sym, exp.Amount),
		})
	}
	writer.Flush()

	return connect.NewResponse(&expensev1.ExportExpensesCSVResponse{
		CsvData: buf.Bytes(),
	}), nil
}

