package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Context key — typed to avoid collisions with other packages.
// ---------------------------------------------------------------------------

type contextKey string

const userIDKey contextKey = "userID"
const sessionIDKey contextKey = "sessionID"

// UserIDFromContext retrieves the authenticated user's ID.
func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

// SessionIDFromContext retrieves the authenticated session's ID.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sessionIDKey).(string)
	return id, ok
}

// ---------------------------------------------------------------------------
// JWT auth interceptor
// ---------------------------------------------------------------------------

// NewAuthInterceptor returns a ConnectRPC unary interceptor that validates a
// Bearer JWT in the "Authorization" header and injects the user ID into the
// request context.
//
// Token format expected from the client:
//
//	Authorization: Bearer <signed-jwt>
//
// The JWT must contain a "sub" claim that is the user's integer ID (as a
// string, per RFC 7519). Example payload:
//
//	{ "sub": "42", "exp": 1720000000 }
//
// The secret must match the value of the JWT_SECRET environment variable
// loaded in config.Load().
func NewAuthInterceptor(secret string, db *sql.DB) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Skip auth check for Login and Register
			if req.Spec().Procedure == "/expense.v1.AuthService/Login" || req.Spec().Procedure == "/expense.v1.AuthService/Register" {
				return next(ctx, req)
			}

			// 1. Extract the raw header value.
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("authorization header is missing"),
				)
			}

			// 2. Expect "Bearer <token>".
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("authorization header must be in the form: Bearer <token>"),
				)
			}
			rawToken := parts[1]

			// 3. Parse and validate the JWT signature + expiry.
			token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
				// Reject tokens signed with unexpected algorithms.
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("invalid or expired token"),
				)
			}

			// 4. Extract user ID from the "sub" claim and session ID from "jti" claim.
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("malformed token claims"),
				)
			}

			sub, err := claims.GetSubject()
			if err != nil || sub == "" {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("token missing subject claim"),
				)
			}

			userID, err := strconv.Atoi(sub)
			if err != nil {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("subject claim is not a valid user ID"),
				)
			}

			// Extract "jti" (session ID)
			jti, ok := claims["jti"].(string)
			if !ok || jti == "" {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("token missing session ID (jti) claim"),
				)
			}

			// 5. Verify the session exists in the database and is not expired
			var exists bool
			err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1 AND expires_at > NOW())", jti).Scan(&exists)
			if err != nil || !exists {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					fmt.Errorf("session invalid or expired"),
				)
			}

			// 6. Inject user ID and session ID into context and continue.
			ctx = context.WithValue(ctx, userIDKey, userID)
			ctx = context.WithValue(ctx, sessionIDKey, jti)
			return next(ctx, req)
		})
	})
}
