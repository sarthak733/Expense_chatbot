package nlp

import (
	"testing"
)

func TestParseTextBrinjalReal(t *testing.T) {
	parsed := ParseText("brinjal for 10 rupees")
	t.Logf("Parsed: %+v", parsed)
	
	if parsed.Title != "Brinjal" {
		t.Errorf("Expected title to be 'Brinjal', got '%s'", parsed.Title)
	}
	if parsed.Amount != 10 {
		t.Errorf("Expected amount to be 10, got %f", parsed.Amount)
	}
	if parsed.Currency != "INR" {
		t.Errorf("Expected currency to be 'INR', got '%s'", parsed.Currency)
	}
	if parsed.Category != "Food" {
		t.Errorf("Expected category to be 'Food', got '%s'", parsed.Category)
	}
}

func TestParseTextDishes(t *testing.T) {
	tests := []struct {
		input            string
		expectedTitle    string
		expectedCategory string
	}{
		{"10 rupees for bruschetta", "Bruschetta", "Food"},
		{"20 rupees for croissant", "Croissant", "Food"},
		{"pasta for 50", "Pasta", "Food"},
		{"chowmein for 40", "Chowmein", "Food"},
	}

	for _, tc := range tests {
		parsed := ParseText(tc.input)
		t.Logf("Input %q -> Parsed: %+v", tc.input, parsed)
		if parsed.Title != tc.expectedTitle {
			t.Errorf("For %q, expected title %q, got %q", tc.input, tc.expectedTitle, parsed.Title)
		}
		if parsed.Category != tc.expectedCategory {
			t.Errorf("For %q, expected category %q, got %q", tc.input, tc.expectedCategory, parsed.Category)
		}
	}
}
