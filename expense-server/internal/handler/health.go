package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"expense-server/internal/response"
)

// Handler holds dependencies required by your HTTP endpoints
type Handler struct {
	DB *sql.DB
}

// New returns a handler instance with dependencies injected
func New(db *sql.DB) *Handler {
	return &Handler{DB: db}
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, "expense-server is running", nil)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	// Ping the DB to make sure it's genuinely healthy
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := "healthy"
	if err := h.DB.PingContext(ctx); err != nil {
		status = "unhealthy (database unreachable)"
		response.JSON(w, http.StatusServiceUnavailable, "error", map[string]string{"status": status})
		return
	}

	response.JSON(w, http.StatusOK, "ok", map[string]string{"status": status})
}
