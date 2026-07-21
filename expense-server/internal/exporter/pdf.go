package exporter

import (
	"bytes"
	"fmt"
	"time"

	expensev1 "expense-server/gen/expense/v1"
	"github.com/jung-kurt/gofpdf/v2"
)

// GenerateExpensesPDF creates a beautifully styled PDF from a list of expenses.
// currency is an ISO 4217 code such as "USD", "INR", "EUR".
func GenerateExpensesPDF(expenses []*expensev1.Expense, username, currency string) ([]byte, error) {
	sym := pdfCurrencySymbol(currency)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 20, 15)
	pdf.AddPage()

	// Colors
	primaryColor := [3]int{44, 62, 80}   // Dark grey-blue (#2C3E50)
	textColor := [3]int{51, 51, 51}      // Charcoal
	lightGrey := [3]int{240, 240, 240}   // Alternating rows
	accentColor := [3]int{52, 152, 219}  // Premium blue highlight

	// --- Header ---
	pdf.SetFillColor(primaryColor[0], primaryColor[1], primaryColor[2])
	pdf.Rect(0, 0, 210, 40, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 18)
	pdf.Text(15, 18, "EXPENSE REPORT")

	pdf.SetFont("Arial", "", 10)
	pdf.Text(15, 25, fmt.Sprintf("Generated for: %s", username))
	pdf.Text(15, 30, fmt.Sprintf("Date: %s", time.Now().Format("2006-01-02 15:04:05")))

	// Spacer below header
	pdf.Ln(28)

	// --- Table Headers ---
	pdf.SetTextColor(primaryColor[0], primaryColor[1], primaryColor[2])
	pdf.SetFont("Arial", "B", 11)

	// Column widths (Total width = 180)
	colIDWidth := 15.0
	colDateWidth := 35.0
	colTitleWidth := 65.0
	colCategoryWidth := 35.0
	colAmountWidth := 30.0

	pdf.CellFormat(colIDWidth, 10, "ID", "B", 0, "L", false, 0, "")
	pdf.CellFormat(colDateWidth, 10, "Date", "B", 0, "L", false, 0, "")
	pdf.CellFormat(colTitleWidth, 10, "Description", "B", 0, "L", false, 0, "")
	pdf.CellFormat(colCategoryWidth, 10, "Category", "B", 0, "L", false, 0, "")
	pdf.CellFormat(colAmountWidth, 10, "Amount", "B", 1, "R", false, 0, "")

	// --- Table Rows ---
	pdf.SetTextColor(textColor[0], textColor[1], textColor[2])
	pdf.SetFont("Arial", "", 10)

	totalAmount := 0.0
	fill := false

	for _, exp := range expenses {
		// Parse and format date
		dateStr := exp.CreatedAt
		parsedTime, err := time.Parse(time.RFC3339, exp.CreatedAt)
		if err == nil {
			dateStr = parsedTime.Format("2006-01-02 15:04")
		}

		// Calculate height dynamically
		h := 8.0

		// Background color for alternating rows
		if fill {
			pdf.SetFillColor(lightGrey[0], lightGrey[1], lightGrey[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		// Check for page overflow
		if pdf.GetY()+h > 270 {
			pdf.AddPage()
			// Redraw table headers on new page
			pdf.SetTextColor(primaryColor[0], primaryColor[1], primaryColor[2])
			pdf.SetFont("Arial", "B", 11)
			pdf.CellFormat(colIDWidth, 10, "ID", "B", 0, "L", false, 0, "")
			pdf.CellFormat(colDateWidth, 10, "Date", "B", 0, "L", false, 0, "")
			pdf.CellFormat(colTitleWidth, 10, "Description", "B", 0, "L", false, 0, "")
			pdf.CellFormat(colCategoryWidth, 10, "Category", "B", 0, "L", false, 0, "")
			pdf.CellFormat(colAmountWidth, 10, "Amount", "B", 1, "R", false, 0, "")
			pdf.SetTextColor(textColor[0], textColor[1], textColor[2])
			pdf.SetFont("Arial", "", 10)
		}

		pdf.CellFormat(colIDWidth, h, fmt.Sprintf("%d", exp.Id), "B", 0, "L", true, 0, "")
		pdf.CellFormat(colDateWidth, h, dateStr, "B", 0, "L", true, 0, "")
		pdf.CellFormat(colTitleWidth, h, exp.Title, "B", 0, "L", true, 0, "")
		pdf.CellFormat(colCategoryWidth, h, exp.Category, "B", 0, "L", true, 0, "")
		pdf.CellFormat(colAmountWidth, h, fmt.Sprintf("%s%.2f", sym, exp.Amount), "B", 1, "R", true, 0, "")

		totalAmount += exp.Amount
		fill = !fill
	}

	// --- Summary Total ---
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(primaryColor[0], primaryColor[1], primaryColor[2])

	// Total Label and Value
	pdf.CellFormat(colIDWidth+colDateWidth+colTitleWidth+colCategoryWidth, 10, "Total Spending:", "", 0, "R", false, 0, "")
	
	pdf.SetTextColor(accentColor[0], accentColor[1], accentColor[2])
	pdf.CellFormat(colAmountWidth, 10, fmt.Sprintf("%s%.2f", sym, totalAmount), "", 1, "R", false, 0, "")

	// --- Footer Signature (Subtle) ---
	pdf.SetTextColor(128, 128, 128)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetY(280)
	pdf.CellFormat(0, 10, fmt.Sprintf("Page %d | Premium Expense Tracker", pdf.PageNo()), "", 0, "C", false, 0, "")

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pdfCurrencySymbol maps an ISO 4217 currency code to a printable symbol.
// Falls back to the code itself for unmapped currencies.
func pdfCurrencySymbol(code string) string {
	symbols := map[string]string{
		"USD": "$", "EUR": "EUR ", "GBP": "GBP ", "INR": "Rs ",
		"JPY": "JPY ", "CNY": "CNY ", "KRW": "KRW ", "CAD": "CA$",
		"AUD": "A$", "NZD": "NZ$", "SGD": "S$", "HKD": "HK$",
		"CHF": "Fr ", "BRL": "R$", "MXN": "MX$", "ZAR": "R ",
		"RUB": "RUB ", "PKR": "Rs ", "BDT": "BDT ", "THB": "THB ",
		"AED": "AED ", "SAR": "SAR ", "TRY": "TRY ", "PLN": "zl ",
		"ILS": "ILS ", "NOK": "kr ", "SEK": "kr ", "DKK": "kr ",
		"IDR": "Rp ", "MYR": "RM ", "PHP": "PHP ", "VND": "VND ",
		"NGN": "NGN ", "COP": "COP$",
	}
	if sym, ok := symbols[code]; ok {
		return sym
	}
	return code + " "
}
