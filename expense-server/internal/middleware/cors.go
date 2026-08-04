package middleware

import (
	"net/http"
	"os"
	"strings"
)

// AllowedOrigins reads origins from CORS_ALLOWED_ORIGINS (comma-separated),
// so prod can lock this down without a code change. Defaults cover local
// frontend dev servers (Vite's default port, plus a couple of common
// alternates).
func AllowedOrigins() []string {
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return []string{
		"http://localhost:5173",
		"http://localhost:4173", // Vite preview
		"http://127.0.0.1:5173",
	}
}

func isAllowedOrigin(origin string) bool {
	for _, o := range AllowedOrigins() {
		if o == origin {
			return true
		}
	}
	return false
}

// CORS allows the browser-based frontend (a different origin/port than
// this server) to call the ConnectRPC endpoints. Connect requests need
// Content-Type, Connect-Protocol-Version, and Authorization allowed
// through, and browsers preflight with OPTIONS before the real request.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type, Authorization, Connect-Protocol-Version, Connect-Timeout-Ms",
			)
			w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
