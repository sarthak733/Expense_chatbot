package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"time"

	"connectrpc.com/connect"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/rs/cors"

	"expense-server/ent"
	"expense-server/gen/expense/v1/expensev1connect"
	"expense-server/internal/config"
	"expense-server/internal/database"
	"expense-server/internal/handler"
	"expense-server/internal/middleware"
	"expense-server/internal/scheduler"
)

func main() {
	cfg := config.Load()

	// Warn early if JWT secret is missing — the auth interceptor will reject
	// every protected request without it, so we surface the problem at startup.
	if cfg.JWTSecret == "" {
		log.Println("WARNING: JWT_SECRET is not set; all protected endpoints will return Unauthenticated")
	}

	// 1. Initialize Database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("database connection pool established successfully")
 
	// Run schema migrations
	if err := database.ApplyMigrations(db, "migrations"); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}
	// Seed default system categories
	if err := database.SeedDefaultCategories(db); err != nil {
		log.Fatalf("failed to seed default categories: %v", err)
	}
	log.Println("database schema initialized and seeded successfully")

	// Start background recurring expense scheduler (runs check every hour by default)
	schedInterval := 1 * time.Hour
	if sVal := os.Getenv("SCHEDULER_INTERVAL"); sVal != "" {
		if dur, err := time.ParseDuration(sVal); err == nil {
			schedInterval = dur
		}
	}
	schedCtx, cancelSched := context.WithCancel(context.Background())
	defer cancelSched()
	scheduler.Start(schedCtx, db, schedInterval)



	// 2. Initialize Ent Client from the existing sql.DB connection pool
	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(drv))
	defer entClient.Close()

	// 3. Initialize Handler (shared implementation of both services)
	h := handler.New(db, entClient)

	// 3. Build the JWT interceptor — only applied to the ExpenseService.
	authInterceptor := middleware.NewAuthInterceptor(cfg.JWTSecret, db)

	// 4. Register routes.
	mux := http.NewServeMux()
	registerRoutes(mux, h, db, cfg.JWTSecret, authInterceptor)

	// 5. Wrap the whole mux with global HTTP-level middleware (logging + recovery).
	var root http.Handler = mux
	root = middleware.Logging(root)
	root = middleware.Recover(root)
	root = middleware.CORS(root)

	// 6. Add CORS so the frontend (e.g. localhost:5173) can reach the backend.
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173", // Vite dev server
			"http://localhost:4173", // Vite preview
			"http://127.0.0.1:5173",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Grpc-Timeout",
			"X-Grpc-Web",
		},
		AllowCredentials: true,
	})
	root = c.Handler(root)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      root,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("server starting on port %s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
 
	cancelSched()
	log.Println("shutdown signal received, draining in-flight requests...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}

	log.Println("server stopped cleanly")
}

// registerRoutes wires up both ConnectRPC services.
//
// Public (no auth):
//   - HealthService  — liveness / readiness probe
//   - Root "/"       — plain HTTP sanity check
//
//   - AuthService    — register, login, logout
//
// Protected (JWT required via authInterceptor):
//   - ExpenseService — full CRUD, scoped to the authenticated user
func registerRoutes(mux *http.ServeMux, h *handler.Handler, db *sql.DB, jwtSecret string, authInterceptor connect.UnaryInterceptorFunc) {
	// --- Public endpoints ---
	mux.HandleFunc("GET /{$}", h.Root) // bare "/" health-check page

	// HealthService: no interceptors
	healthPath, healthHandler := expensev1connect.NewHealthServiceHandler(h)
	mux.Handle(healthPath, healthHandler)

	// AuthService: Register/Login are public. Logout is public but technically fails without context. Wait, let's put AuthService under authInterceptor? No, Register/Login would fail. We'll handle Logout internally or put Logout on ExpenseService. Actually, AuthService is better without global authInterceptor, Logout will manually read the token or we can split it.
	// Since we defined Logout to require auth, we can just apply authInterceptor to AuthService? No, Login/Register would fail.
	// Let's mount AuthService without the interceptor, and let Logout manually check the session, or it's fine. Wait, `auth.go` handler uses `middleware.SessionIDFromContext(ctx)`. If there's no interceptor, it will fail with "not authenticated". That's correct.
	authH := &handler.AuthHandler{
		DB:        db,
		JWTSecret: jwtSecret,
	}
	// We apply authInterceptor ONLY to Logout? Connect-Go allows method-specific interceptors or we just apply it to ExpenseService and for Logout, we do it manually. 
	// To keep it simple, we mount AuthService without interceptors, but wait, `middleware.SessionIDFromContext` relies on the interceptor! 
	// I'll just mount AuthService WITH interceptors but only for Logout. Connect doesn't easily do per-method without custom logic. 
	// I will just mount AuthService without interceptor, and update `Logout` to parse the token manually.
	
	// Better yet, let's just mount AuthService WITH the interceptor, but the interceptor only checks if the method is NOT Login or Register.
	// That's standard practice. I will update NewAuthInterceptor.
	
	authPath, authHandler := expensev1connect.NewAuthServiceHandler(authH, connect.WithInterceptors(authInterceptor))
	mux.Handle(authPath, authHandler)

	// --- Protected endpoints (auth interceptor applied) ---
	// ExpenseService: JWT auth interceptor gates every RPC in this service.
	expensePath, expenseHandler := expensev1connect.NewExpenseServiceHandler(
		h,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(expensePath, expenseHandler)

	// UserService: JWT auth interceptor gates every RPC in this service.
	userPath, userHandler := expensev1connect.NewUserServiceHandler(
		h,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(userPath, userHandler)
}
