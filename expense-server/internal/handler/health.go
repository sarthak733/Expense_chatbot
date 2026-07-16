package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"expense-server/ent"
	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/response"
)

// Handler holds shared dependencies (DB) for all service implementations.
// It satisfies both the generated ExpenseServiceHandler and HealthServiceHandler
// interfaces — keeping the setup in main.go simple (one New() call).
type Handler struct {
	DB        *sql.DB
	EntClient *ent.Client
}

// New returns a Handler with the database connection pool and Ent Client injected.
func New(db *sql.DB, entClient *ent.Client) *Handler {
	return &Handler{
		DB:        db,
		EntClient: entClient,
	}
}

// ---------------------------------------------------------------------------
// Plain HTTP handlers (not ConnectRPC — these are simple REST endpoints)
// ---------------------------------------------------------------------------

// Root is a bare HTTP GET "/" sanity check — no auth required.
func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, "expense-server is running", nil)
}

// ---------------------------------------------------------------------------
// HealthService — ConnectRPC implementation
// ---------------------------------------------------------------------------

// HealthCheck pings the database and reports overall server health.
// This endpoint is deliberately unauthenticated so liveness probes
// (Kubernetes, load balancers, etc.) never need a token.
func (h *Handler) HealthCheck(
	ctx context.Context,
	_ *connect.Request[expensev1.HealthCheckRequest],
) (*connect.Response[expensev1.HealthCheckResponse], error) {

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	status := "healthy"
	msg := "ok"

	if err := h.DB.PingContext(pingCtx); err != nil {
		status = "unhealthy"
		msg = "database unreachable"
	}

	return connect.NewResponse(&expensev1.HealthCheckResponse{
		Status:  status,
		Message: msg,
	}), nil
}
