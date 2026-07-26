package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"expense-server/ent"
	"expense-server/gen/expense/v1/expensev1connect"
	"expense-server/internal/database"
	"expense-server/internal/handler"
	"expense-server/internal/middleware"
)

const (
	testJWTSecret = "test-jwt-secret-key-that-is-long-enough-32-bytes"
	testDBURL     = "postgres://postgres:sergtsop@localhost:5432/expense_tracker_dev?sslmode=disable"
)

func setupTestServer(t *testing.T) (*httptest.Server, *sql.DB, *ent.Client) {
	// Connect to test database
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = testDBURL
	}

	db, err := database.New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Apply migrations if not already applied
	err = database.ApplyMigrations(db, "../migrations")
	if err != nil {
		db.Close()
		t.Fatalf("failed to apply migrations in test: %v", err)
	}

	// Clean tables before running tests
	_, err = db.Exec("TRUNCATE TABLE sessions, chat_messages, expenses, budgets, recurring_expenses, categories, users CASCADE")
	if err != nil {
		db.Close()
		t.Fatalf("failed to truncate test tables: %v", err)
	}

	// Initialize Ent client
	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))

	// Seed default categories
	err = database.SeedDefaultCategories(db)
	if err != nil {
		entClient.Close()
		db.Close()
		t.Fatalf("failed to seed categories in test: %v", err)
	}

	// Setup handler and routes
	h := handler.New(db, entClient)
	authInterceptor := middleware.NewAuthInterceptor(testJWTSecret, db)

	mux := http.NewServeMux()
	
	// Register health check
	healthPath, healthHandler := expensev1connect.NewHealthServiceHandler(h)
	mux.Handle(healthPath, healthHandler)

	// Register auth service
	authH := &handler.AuthHandler{
		DB:        db,
		JWTSecret: testJWTSecret,
	}
	authPath, authHandler := expensev1connect.NewAuthServiceHandler(authH, connect.WithInterceptors(authInterceptor))
	mux.Handle(authPath, authHandler)

	// Register expense service
	expensePath, expenseHandler := expensev1connect.NewExpenseServiceHandler(
		h,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(expensePath, expenseHandler)

	// Register user service
	userPath, userHandler := expensev1connect.NewUserServiceHandler(
		h,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(userPath, userHandler)

	server := httptest.NewServer(mux)
	return server, db, entClient
}

func TestHealthCheck(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	resp, err := http.Post(
		fmt.Sprintf("%s/expense.v1.HealthService/HealthCheck", server.URL),
		"application/json",
		bytes.NewBuffer([]byte("{}")),
	)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", result["status"])
	}
}

func TestAuthFlow(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	// 1. Register User
	registerPayload, _ := json.Marshal(map[string]string{
		"username": "tester",
		"password": "securepassword123",
	})
	resp, err := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Register", server.URL),
		"application/json",
		bytes.NewBuffer(registerPayload),
	)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected register status 200, got %d", resp.StatusCode)
	}

	var registerRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerRes)

	token, ok := registerRes["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected token in register response")
	}

	// 2. Login User
	loginPayload, _ := json.Marshal(map[string]string{
		"username": "tester",
		"password": "securepassword123",
	})
	resp2, err := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Login", server.URL),
		"application/json",
		bytes.NewBuffer(loginPayload),
	)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected login status 200, got %d", resp2.StatusCode)
	}

	var loginRes map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&loginRes)
	loginToken := loginRes["token"].(string)
	if loginToken == "" {
		t.Fatal("expected token in login response")
	}
}

func TestExpenseFlow(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	// Register user to get JWT token
	registerPayload, _ := json.Marshal(map[string]string{
		"username": "tester",
		"password": "securepassword123",
	})
	resp, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Register", server.URL),
		"application/json",
		bytes.NewBuffer(registerPayload),
	)
	var registerRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerRes)
	resp.Body.Close()
	token := registerRes["token"].(string)

	// Create Category
	catPayload, _ := json.Marshal(map[string]string{
		"name":  "Food & Groceries",
		"color": "#FF5733",
	})
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.ExpenseService/CreateCategory", server.URL),
		bytes.NewBuffer(catPayload),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	respCat, err := client.Do(req)
	if err != nil {
		t.Fatalf("create category request failed: %v", err)
	}
	defer respCat.Body.Close()

	if respCat.StatusCode != http.StatusOK {
		t.Errorf("expected create category status 200, got %d", respCat.StatusCode)
	}

	var catRes map[string]interface{}
	json.NewDecoder(respCat.Body).Decode(&catRes)
	categoryField, ok := catRes["category"].(map[string]interface{})
	if !ok {
		t.Fatal("category key missing or wrong type in create category response")
	}
	catID := int(categoryField["id"].(float64))

	// Create Expense
	expPayload, _ := json.Marshal(map[string]interface{}{
		"title":       "Weekly Groceries",
		"amount":      75.50,
		"category_id": catID,
	})
	req2, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.ExpenseService/CreateExpense", server.URL),
		bytes.NewBuffer(expPayload),
	)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)

	respExp, err := client.Do(req2)
	if err != nil {
		t.Fatalf("create expense request failed: %v", err)
	}
	defer respExp.Body.Close()

	if respExp.StatusCode != http.StatusOK {
		t.Errorf("expected create expense status 200, got %d", respExp.StatusCode)
	}

	var expRes map[string]interface{}
	json.NewDecoder(respExp.Body).Decode(&expRes)
	expenseField := expRes["expense"].(map[string]interface{})
	if expenseField["title"] != "Weekly Groceries" {
		t.Errorf("expected expense title 'Weekly Groceries', got %v", expenseField["title"])
	}
}

func TestUserProfileFlow(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	// Register user to get JWT token
	registerPayload, _ := json.Marshal(map[string]string{
		"username": "profiletester",
		"password": "password123",
	})
	resp, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Register", server.URL),
		"application/json",
		bytes.NewBuffer(registerPayload),
	)
	var registerRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerRes)
	resp.Body.Close()
	token := registerRes["token"].(string)

	client := &http.Client{}

	// 1. Get Profile (Default settings check)
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.UserService/GetProfile", server.URL),
		bytes.NewBuffer([]byte("{}")),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	respGet, err := client.Do(req)
	if err != nil {
		t.Fatalf("get profile request failed: %v", err)
	}
	defer respGet.Body.Close()

	if respGet.StatusCode != http.StatusOK {
		t.Fatalf("expected get profile status 200, got %d", respGet.StatusCode)
	}

	var getRes map[string]interface{}
	json.NewDecoder(respGet.Body).Decode(&getRes)
	profile := getRes["profile"].(map[string]interface{})

	if profile["currency"] != "USD" {
		t.Errorf("expected default currency USD, got %v", profile["currency"])
	}
	if profile["theme"] != "system" {
		t.Errorf("expected default theme system, got %v", profile["theme"])
	}
	if profile["weeklyStart"] != "monday" {
		t.Errorf("expected default weeklyStart monday, got %v", profile["weeklyStart"])
	}

	// 2. Update Profile Settings (using camelCase for protobuf fields)
	updatePayload, _ := json.Marshal(map[string]interface{}{
		"email":                "jane@test.com",
		"firstName":            "Jane",
		"lastName":             "Doe",
		"currency":             "EUR",
		"theme":                "dark",
		"weeklyStart":          "sunday",
		"monthlyStartDay":      5,
		"budgetAlertThreshold": 95.0,
	})
	req2, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.UserService/UpdateProfile", server.URL),
		bytes.NewBuffer(updatePayload),
	)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)

	respUpdate, err := client.Do(req2)
	if err != nil {
		t.Fatalf("update profile request failed: %v", err)
	}
	defer respUpdate.Body.Close()

	if respUpdate.StatusCode != http.StatusOK {
		t.Fatalf("expected update profile status 200, got %d", respUpdate.StatusCode)
	}

	var updateRes map[string]interface{}
	json.NewDecoder(respUpdate.Body).Decode(&updateRes)
	profile2 := updateRes["profile"].(map[string]interface{})

	if profile2["email"] != "jane@test.com" || profile2["firstName"] != "Jane" || profile2["lastName"] != "Doe" {
		t.Errorf("profile name/email fields did not update correctly: got email=%v, firstName=%v, lastName=%v", profile2["email"], profile2["firstName"], profile2["lastName"])
	}
	if profile2["currency"] != "EUR" {
		t.Errorf("expected updated currency EUR, got %v", profile2["currency"])
	}
	if profile2["theme"] != "dark" {
		t.Errorf("expected updated theme dark, got %v", profile2["theme"])
	}
	if int(profile2["monthlyStartDay"].(float64)) != 5 {
		t.Errorf("expected updated monthlyStartDay 5, got %v", profile2["monthlyStartDay"])
	}

	// 3. Update Password
	pwPayload, _ := json.Marshal(map[string]string{
		"old_password": "password123",
		"new_password": "newpassword456",
	})
	req3, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.UserService/UpdatePassword", server.URL),
		bytes.NewBuffer(pwPayload),
	)
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+token)

	respPW, err := client.Do(req3)
	if err != nil {
		t.Fatalf("update password request failed: %v", err)
	}
	defer respPW.Body.Close()

	if respPW.StatusCode != http.StatusOK {
		t.Fatalf("expected update password status 200, got %d", respPW.StatusCode)
	}

	// 4. Verify login with old password fails, and new password succeeds
	// Test Login with OLD password (should fail)
	loginFailPayload, _ := json.Marshal(map[string]string{
		"username": "profiletester",
		"password": "password123",
	})
	respLoginFail, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Login", server.URL),
		"application/json",
		bytes.NewBuffer(loginFailPayload),
	)
	if respLoginFail.StatusCode == http.StatusOK {
		t.Errorf("expected login with old password to fail, but it succeeded")
	}
	respLoginFail.Body.Close()

	// Test Login with NEW password (should succeed)
	loginSuccessPayload, _ := json.Marshal(map[string]string{
		"username": "profiletester",
		"password": "newpassword456",
	})
	respLoginSuccess, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Login", server.URL),
		"application/json",
		bytes.NewBuffer(loginSuccessPayload),
	)
	if respLoginSuccess.StatusCode != http.StatusOK {
		t.Errorf("expected login with new password to succeed, got %d", respLoginSuccess.StatusCode)
	}
	respLoginSuccess.Body.Close()

	// 5. Delete Profile
	req4, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.UserService/DeleteProfile", server.URL),
		bytes.NewBuffer([]byte("{}")),
	)
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer "+token)

	respDelete, err := client.Do(req4)
	if err != nil {
		t.Fatalf("delete profile request failed: %v", err)
	}
	defer respDelete.Body.Close()

	if respDelete.StatusCode != http.StatusOK {
		t.Fatalf("expected delete profile status 200, got %d", respDelete.StatusCode)
	}

	// 6. Verify profile is gone (should get unauthenticated because session is cascading deleted)
	req5, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/expense.v1.UserService/GetProfile", server.URL),
		bytes.NewBuffer([]byte("{}")),
	)
	req5.Header.Set("Content-Type", "application/json")
	req5.Header.Set("Authorization", "Bearer "+token)

	respGetGone, _ := client.Do(req5)
	if respGetGone.StatusCode == http.StatusOK {
		t.Errorf("expected get profile to fail after delete, but got 200")
	}
	respGetGone.Body.Close()
}

func TestMultiCurrencyFlow(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	// 1. Register User
	registerPayload, _ := json.Marshal(map[string]string{
		"username": "currency_tester",
		"password": "password123",
	})
	resp, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Register", server.URL),
		"application/json",
		bytes.NewBuffer(registerPayload),
	)
	var registerRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerRes)
	resp.Body.Close()
	token := registerRes["token"].(string)

	client := &http.Client{}

	// 2. Create Expense in INR
	expINR, _ := json.Marshal(map[string]interface{}{
		"title":    "Lunch in Delhi",
		"amount":   250.00,
		"currency": "INR",
	})
	req1, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/CreateExpense", server.URL), bytes.NewBuffer(expINR))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("create expense INR failed: %v", err)
	}
	var res1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&res1)
	resp1.Body.Close()
	expenseINR := res1["expense"].(map[string]interface{})
	if expenseINR["currency"] != "INR" {
		t.Errorf("expected created expense currency to be INR, got %v", expenseINR["currency"])
	}

	// 3. Create Expense in USD
	expUSD, _ := json.Marshal(map[string]interface{}{
		"title":    "Lunch in NY",
		"amount":   15.50,
		"currency": "USD",
	})
	req2, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/CreateExpense", server.URL), bytes.NewBuffer(expUSD))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("create expense USD failed: %v", err)
	}
	var res2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&res2)
	resp2.Body.Close()
	expenseUSD := res2["expense"].(map[string]interface{})
	if expenseUSD["currency"] != "USD" {
		t.Errorf("expected created expense currency to be USD, got %v", expenseUSD["currency"])
	}

	// 4. Update Profile Preferred Currency to EUR
	updateProfile, _ := json.Marshal(map[string]interface{}{
		"currency": "EUR",
	})
	req3, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.UserService/UpdateProfile", server.URL), bytes.NewBuffer(updateProfile))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+token)
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("update profile failed: %v", err)
	}
	resp3.Body.Close()

	// 5. Get List of Expenses and verify currencies are unchanged
	req4, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/ListExpenses", server.URL), bytes.NewBuffer([]byte("{}")))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer "+token)
	resp4, err := client.Do(req4)
	if err != nil {
		t.Fatalf("list expenses failed: %v", err)
	}
	var res4 map[string]interface{}
	json.NewDecoder(resp4.Body).Decode(&res4)
	resp4.Body.Close()

	expenses := res4["expenses"].([]interface{})
	if len(expenses) != 2 {
		t.Fatalf("expected 2 expenses, got %d", len(expenses))
	}

	// List retrieves in descending order of created_at. Since USD was created second, it is first in the list.
	firstExp := expenses[0].(map[string]interface{})
	secondExp := expenses[1].(map[string]interface{})

	if firstExp["currency"] != "USD" {
		t.Errorf("expected first expense currency to remain USD, got %v", firstExp["currency"])
	}
	if secondExp["currency"] != "INR" {
		t.Errorf("expected second expense currency to remain INR, got %v", secondExp["currency"])
	}
}

func TestHistoryBasedCategoryResolution(t *testing.T) {
	server, db, entClient := setupTestServer(t)
	defer server.Close()
	defer entClient.Close()
	defer db.Close()

	// 1. Register User
	registerPayload, _ := json.Marshal(map[string]string{
		"username": "history_tester",
		"password": "password123",
	})
	resp, _ := http.Post(
		fmt.Sprintf("%s/expense.v1.AuthService/Register", server.URL),
		"application/json",
		bytes.NewBuffer(registerPayload),
	)
	var registerRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerRes)
	resp.Body.Close()
	token := registerRes["token"].(string)

	client := &http.Client{}

	// 2. Create custom category "Luxuries"
	catPayload, _ := json.Marshal(map[string]string{
		"name":  "Luxuries",
		"color": "#990099",
	})
	reqCat, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/CreateCategory", server.URL), bytes.NewBuffer(catPayload))
	reqCat.Header.Set("Content-Type", "application/json")
	reqCat.Header.Set("Authorization", "Bearer "+token)
	respCat, _ := client.Do(reqCat)
	var catRes map[string]interface{}
	json.NewDecoder(respCat.Body).Decode(&catRes)
	respCat.Body.Close()
	luxuriesCatID := int(catRes["category"].(map[string]interface{})["id"].(float64))

	// 3. Log a specific custom dish/item "Truffle Fries" in "Luxuries" category manually
	expPayload, _ := json.Marshal(map[string]interface{}{
		"title":       "Truffle Fries",
		"amount":      18.50,
		"category_id": luxuriesCatID,
	})
	reqExp, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/CreateExpense", server.URL), bytes.NewBuffer(expPayload))
	reqExp.Header.Set("Content-Type", "application/json")
	reqExp.Header.Set("Authorization", "Bearer "+token)
	respExp, _ := client.Do(reqExp)
	respExp.Body.Close()

	// 4. Send a chatbot chat message logging "Truffle Fries" again, but without specifying category
	// Chatbot uses SendChatMessage which parses "truffle fries for 20 rupees" -> title "Truffle Fries", fallback category "Others".
	// But since the user previously manually logged "Truffle Fries" under "Luxuries", it should automatically resolve to "Luxuries"!
	chatPayload, _ := json.Marshal(map[string]string{
		"message_text": "truffle fries for 20 rupees",
	})
	reqChat, _ := http.NewRequest("POST", fmt.Sprintf("%s/expense.v1.ExpenseService/SendChatMessage", server.URL), bytes.NewBuffer(chatPayload))
	reqChat.Header.Set("Content-Type", "application/json")
	reqChat.Header.Set("Authorization", "Bearer "+token)
	respChat, err := client.Do(reqChat)
	if err != nil {
		t.Fatalf("chat request failed: %v", err)
	}
	var chatRes map[string]interface{}
	json.NewDecoder(respChat.Body).Decode(&chatRes)
	respChat.Body.Close()

	botResponse := chatRes["botResponse"].(map[string]interface{})
	replyText := botResponse["messageText"].(string)

	expectedReply := "Successfully logged an expense: ₹20.00 for 'Truffle fries' in category 'Luxuries'."
	if replyText != expectedReply {
		t.Errorf("Expected bot reply %q, got %q", expectedReply, replyText)
	}
}
