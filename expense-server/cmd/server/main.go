package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"expense-server/internal/config"
	"expense-server/internal/database" // <-- Add this
	"expense-server/internal/handler"
	"expense-server/internal/middleware"
)

func main() {
	cfg := config.Load()

	// 1. Initialize Database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("database connection pool established successfully")

	// 2. Initialize Handler instance with DB injected
	h := handler.New(db)

	mux := http.NewServeMux()
	registerRoutes(mux, h) // Pass h to the router

	var root http.Handler = mux
	root = middleware.Logging(root)
	root = middleware.Recover(root)

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

	log.Println("shutdown signal received, draining in-flight requests...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}

	log.Println("server stopped cleanly")
}

// Accept the *handler.Handler dependency here
func registerRoutes(mux *http.ServeMux, h *handler.Handler) {
	mux.HandleFunc("GET /{$}", h.Root)
	mux.HandleFunc("GET /health", h.Health)

	// When you implement CRUD handlers later, they will look like this:
	mux.HandleFunc("POST /expenses", h.CreateExpense)
	mux.HandleFunc("GET /expenses", h.ListExpenses)
	mux.HandleFunc("GET /expenses/{id}", h.GetExpense)
	mux.HandleFunc("PUT /expenses/{id}", h.UpdateExpense)
	mux.HandleFunc("DELETE /expenses/{id}", h.DeleteExpense)

}
