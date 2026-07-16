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
