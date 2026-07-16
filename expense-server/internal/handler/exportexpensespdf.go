package handler

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/exporter"
	"expense-server/internal/middleware"
)

func (h *Handler) ExportExpensesPDF(
	ctx context.Context,
	req *connect.Request[expensev1.ExportExpensesRequest],
) (*connect.Response[expensev1.ExportExpensesPDFResponse], error) {

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Get username for PDF branding
	var username string
	_ = h.DB.QueryRowContext(ctx, "SELECT username FROM users WHERE id = $1", userID).Scan(&username)

	listReq := &connect.Request[expensev1.ListExpensesRequest]{
		Msg: &expensev1.ListExpensesRequest{
			StartDate: req.Msg.StartDate,
			EndDate:   req.Msg.EndDate,
			Limit:     100000,
		},
	}
	res, err := h.ListExpenses(ctx, listReq)
	if err != nil {
		return nil, err
	}

	pdfData, err := exporter.GenerateExpensesPDF(res.Msg.Expenses, username)
	if err != nil {
		log.Printf("ERROR generating PDF: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate PDF"))
	}

	return connect.NewResponse(&expensev1.ExportExpensesPDFResponse{
		PdfData: pdfData,
	}), nil
}
