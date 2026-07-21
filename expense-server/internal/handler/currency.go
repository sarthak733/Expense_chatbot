package handler

import (
	"context"
	"database/sql"
	"fmt"
)

// getUserCurrency fetches the preferred currency code for the given user.
// Returns "USD" as a safe fallback if the query fails or the field is empty.
func getUserCurrency(ctx context.Context, db *sql.DB, userID int) (string, error) {
	var currency string
	err := db.QueryRowContext(ctx, "SELECT currency FROM users WHERE id = $1", userID).Scan(&currency)
	if err != nil {
		return "USD", fmt.Errorf("getUserCurrency: %w", err)
	}
	if currency == "" {
		return "USD", nil
	}
	return currency, nil
}

// currencySymbol returns the display symbol for a given ISO 4217 currency code.
// Falls back to the code itself (e.g. "THB ") for unmapped currencies.
func currencySymbol(code string) string {
	symbols := map[string]string{
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"INR": "₹",
		"JPY": "¥",
		"CNY": "¥",
		"KRW": "₩",
		"CAD": "CA$",
		"AUD": "A$",
		"NZD": "NZ$",
		"SGD": "S$",
		"HKD": "HK$",
		"CHF": "Fr",
		"BRL": "R$",
		"MXN": "MX$",
		"ZAR": "R",
		"RUB": "₽",
		"PKR": "₨",
		"BDT": "৳",
		"THB": "฿",
		"AED": "د.إ",
		"SAR": "﷼",
		"TRY": "₺",
		"PLN": "zł",
		"ILS": "₪",
		"NOK": "kr",
		"SEK": "kr",
		"DKK": "kr",
		"IDR": "Rp",
		"MYR": "RM",
		"PHP": "₱",
		"VND": "₫",
		"EGP": "E£",
		"NGN": "₦",
		"COP": "$",
		"ARS": "$",
		"CLP": "$",
	}
	if sym, ok := symbols[code]; ok {
		return sym
	}
	return code + " "
}
