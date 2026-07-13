package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"expense-server/internal/response"
)

// Recover wraps a handler so that a panic in any request (nil pointer,
// index out of range, whatever) turns into a clean 500 response instead
// of crashing the entire server process and taking down every other
// in-flight request with it.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				response.Error(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
