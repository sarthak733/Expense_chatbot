package nlp

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedExpense represents the output of the NLP parser.
type ParsedExpense struct {
	Title    string
	Amount   float64
	Category string
	Date     time.Time
	Currency string // detected ISO 4217 code, or "" if not mentioned
}

// DetectCurrency attempts to extract an explicit currency from natural language text.
// Returns the ISO 4217 code (e.g. "USD") or "" if nothing recognised.
func DetectCurrency(text string) string {
	lower := strings.ToLower(text)

	currencyMap := []struct {
		keywords []string
		code     string
	}{
		{[]string{"usd", "dollar", "dollars", "buck", "bucks", "$"}, "USD"},
		{[]string{"inr", "rupee", "rupees", "rs", "₹"}, "INR"},
		{[]string{"eur", "euro", "euros", "€"}, "EUR"},
		{[]string{"gbp", "pound", "pounds", "sterling", "£"}, "GBP"},
		{[]string{"jpy", "yen", "¥"}, "JPY"},
		{[]string{"cad", "canadian dollar", "ca$"}, "CAD"},
		{[]string{"aud", "australian dollar", "a$"}, "AUD"},
		{[]string{"sgd", "singapore dollar", "s$"}, "SGD"},
		{[]string{"aed", "dirham", "dirhams"}, "AED"},
		{[]string{"pkr", "pakistani rupee", "pakistani rupees"}, "PKR"},
	}

	for _, entry := range currencyMap {
		for _, kw := range entry.keywords {
			// Use word-boundary match for multi-char keywords, substring for symbols
			if len([]rune(kw)) == 1 {
				if strings.Contains(text, kw) {
					return entry.code
				}
			} else {
				rx := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
				if rx.MatchString(lower) {
					return entry.code
				}
			}
		}
	}
	return ""
}

// ParseText processes a natural language string and extracts expense metadata.
func ParseText(text string) ParsedExpense {
	now := time.Now()
	parsed := ParsedExpense{
		Date:     now,
		Category: "Others",
		Currency: DetectCurrency(text),
	}

	// 1. Extract Amount
	// Match pattern: $X, X dollars, X bucks, X usd, or just X.
	// We check for numbers with decimal or integers.
	amountRx := regexp.MustCompile(`(?i)(?:\$|usd|dollars?|bucks)?\s*(\d+(?:\.\d{2})?)\s*(?:\$|usd|dollars?|bucks)?`)
	matches := amountRx.FindAllStringSubmatch(text, -1)
	
	var amountVal float64
	var rawAmountMatch string
	
	// Find the most likely number (highest value, or first non-date match)
	for _, m := range matches {
		if len(m) > 1 {
			val, err := strconv.ParseFloat(m[1], 64)
			if err == nil && val > 0 {
				amountVal = val
				rawAmountMatch = m[0]
				break
			}
		}
	}
	parsed.Amount = amountVal

	// 2. Extract Relative Date
	lowerText := strings.ToLower(text)
	dateMatched := false

	// Match "yesterday"
	if strings.Contains(lowerText, "yesterday") {
		parsed.Date = now.AddDate(0, 0, -1)
		dateMatched = true
	} else if strings.Contains(lowerText, "today") {
		parsed.Date = now
		dateMatched = true
	} else if strings.Contains(lowerText, "tomorrow") {
		parsed.Date = now.AddDate(0, 0, 1)
		dateMatched = true
	} else if strings.Contains(lowerText, "last week") {
		parsed.Date = now.AddDate(0, 0, -7)
		dateMatched = true
	} else if strings.Contains(lowerText, "last month") {
		parsed.Date = now.AddDate(0, -1, 0)
		dateMatched = true
	} else {
		// Match "X days ago"
		daysAgoRx := regexp.MustCompile(`\b(\d+)\s+days?\s+ago\b`)
		if m := daysAgoRx.FindStringSubmatch(lowerText); len(m) > 1 {
			days, err := strconv.Atoi(m[1])
			if err == nil {
				parsed.Date = now.AddDate(0, 0, -days)
				dateMatched = true
			}
		}
	}

	// Match "last <weekday>"
	if !dateMatched {
		weekdayRx := regexp.MustCompile(`\blast\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`)
		if m := weekdayRx.FindStringSubmatch(lowerText); len(m) > 1 {
			parsed.Date = lastWeekday(m[1], now)
		}
	}

	// 3. Clean text to extract Title
	// Strip out date descriptions, amount strings, and words like "spent", "buy", "for", "on".
	cleaned := text
	
	// Remove amount match
	if rawAmountMatch != "" {
		cleaned = strings.Replace(cleaned, rawAmountMatch, " ", 1)
	}
	
	// Remove date expressions
	datePatterns := []string{
		`yesterday`, `today`, `tomorrow`, `last week`, `last month`,
		`\b\d+\s+days?\s+ago\b`,
		`\blast\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`,
	}
	for _, p := range datePatterns {
		rx := regexp.MustCompile("(?i)" + p)
		cleaned = rx.ReplaceAllString(cleaned, "")
	}

	// Remove currency words so they don't end up in the title
	currencyPatterns := []string{
		`\b(usd|inr|eur|gbp|jpy|cad|aud|sgd|aed|pkr)\b`,
		`\b(dollars?|rupees?|euros?|pounds?|yen|bucks?|dirham|dirhams?)\b`,
	}
	for _, p := range currencyPatterns {
		rx := regexp.MustCompile("(?i)" + p)
		cleaned = rx.ReplaceAllString(cleaned, " ")
	}

	// Strip out filler words at the beginning/end
	fillerRx := regexp.MustCompile(`(?i)\b(spent|buy|bought|paid|paying|for|on|at|a|an|the|some|in|of)\b`)
	cleaned = fillerRx.ReplaceAllString(cleaned, " ")

	// Cleanup whitespace
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	// Fallback title if clean title is empty
	if cleaned == "" {
		parsed.Title = "Quick Expense"
	} else {
		// Capitalize first letter
		parsed.Title = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	// 4. Map Title keywords to default Categories
	parsed.Category = classifyCategory(parsed.Title)

	return parsed
}


func lastWeekday(dayStr string, relativeTo time.Time) time.Time {
	wdMap := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	targetWd, ok := wdMap[strings.ToLower(dayStr)]
	if !ok {
		return relativeTo
	}
	diff := int(relativeTo.Weekday() - targetWd)
	if diff <= 0 {
		diff += 7
	}
	return relativeTo.AddDate(0, 0, -diff)
}

func classifyCategory(title string) string {
	lowerTitle := strings.ToLower(title)
	
	// Keyword map
	categoryKeywords := map[string][]string{
		"Food": {
			"coffee", "starbucks", "mcdonald", "lunch", "dinner", "breakfast", "brunch", "pizza", 
			"groceries", "grocery", "supermarket", "food", "restaurant", "cafe", "burger", "tea", 
			"drink", "drinks", "swiggy", "zomato", "kfc", "dominos", "subway", "bakery", "milk", 
			"bread", "eggs", "egg", "chicken", "meat", "fish", "rice", "wheat", "flour", "sugar", 
			"salt", "oil", "butter", "cheese", "snack", "snacks", "biscuit", "biscuits", "chocolate", 
			"candy", "juice", "soda", "brinjal", "eggplant", "vegetable", "vegetables", "fruit", "fruits",
			"apple", "banana", "beer", "wine", "liquor", "booze", "croissant", "croissants", "bruschetta",
			"pasta", "spaghetti", "lasagna", "salad", "soup", "steak", "sushi", "taco", "tacos",
			"burrito", "nachos", "curry", "naan", "roti", "biryani", "noodles", "ramen", "sandwich",
			"sandwiches", "waffle", "pancake", "toast", "bacon", "sausage", "donut", "donuts",
			"muffin", "cake", "pastry", "cookie", "cookies", "ice cream", "dessert", "chowmein",
			"chow mein", "momo", "momos", "maggi", "samosa", "paneer", "tandoori", "kabab", "kebab", "tikka",
		},
		"Transport": {
			"uber", "lyft", "taxi", "cab", "cabs", "bus", "train", "flight", "flights", "plane", 
			"gas", "fuel", "subway", "metro", "ticket", "tickets", "ride", "commute", "parking", 
			"toll", "tolls", "fare", "petrol", "diesel",
		},
		"Utilities": {
			"electricity", "rent", "water", "internet", "wifi", "electric", "bill", "bills", 
			"phone", "mobile", "power", "subscription", "subscriptions", "sewer", "trash", 
			"garbage", "heating", "insurance",
		},
		"Entertainment": {
			"movie", "movies", "cinema", "netflix", "spotify", "youtube premium", "icloud", 
			"google one", "game", "games", "gaming", "steam", "xbox", "playstation", "nintendo", 
			"concert", "concerts", "gig", "theatre", "museum", "play", "pub", "pubs", "club", 
			"clubs", "bar", "bars", "party", "parties", "show", "shows",
		},
		"Shopping": {
			"amazon", "ebay", "walmart", "target", "myntra", "flipkart", "clothes", "clothing", 
			"shoes", "shoe", "shirt", "pants", "jeans", "jacket", "dress", "mall", "shopping", 
			"store", "boutique", "gift", "gifts", "toy", "toys", "electronics", "gadget", 
			"gadgets", "furniture",
		},
	}

	for cat, keywords := range categoryKeywords {
		for _, kw := range keywords {
			// Check if keyword is a whole word or substring
			if strings.Contains(lowerTitle, kw) {
				return cat
			}
		}
	}

	return "Others"
}
