package handler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	expensev1 "expense-server/gen/expense/v1"
	"expense-server/internal/middleware"
)

var _ interface {
	Register(context.Context, *connect.Request[expensev1.RegisterRequest]) (*connect.Response[expensev1.RegisterResponse], error)
	Login(context.Context, *connect.Request[expensev1.LoginRequest]) (*connect.Response[expensev1.LoginResponse], error)
	Logout(context.Context, *connect.Request[expensev1.LogoutRequest]) (*connect.Response[expensev1.LogoutResponse], error)
} = (*AuthHandler)(nil)

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret string
}

func (h *AuthHandler) Register(
	ctx context.Context,
	req *connect.Request[expensev1.RegisterRequest],
) (*connect.Response[expensev1.RegisterResponse], error) {
	if req.Msg.Username == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("username and password are required"))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password"))
	}

	var userID int32
	var createdAt time.Time
	err = h.DB.QueryRowContext(ctx, "INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id, created_at", req.Msg.Username, string(hash)).Scan(&userID, &createdAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("username already exists"))
	}

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = h.DB.ExecContext(ctx, "INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)", sessionID, userID, expiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create session"))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": fmt.Sprintf("%d", userID),
		"jti": sessionID,
		"exp": expiresAt.Unix(),
	})
	signedToken, _ := token.SignedString([]byte(h.JWTSecret))

	return connect.NewResponse(&expensev1.RegisterResponse{
		Message: "User registered successfully",
		User: &expensev1.User{
			Id:        userID,
			Username:  req.Msg.Username,
			CreatedAt: formatTime(createdAt),
		},
		Token:     signedToken,
	}), nil
}

func (h *AuthHandler) Login(
	ctx context.Context,
	req *connect.Request[expensev1.LoginRequest],
) (*connect.Response[expensev1.LoginResponse], error) {
	var userID int32
	var hash string
	var createdAt time.Time
	err := h.DB.QueryRowContext(ctx, "SELECT id, password_hash, created_at FROM users WHERE username = $1", req.Msg.Username).Scan(&userID, &hash, &createdAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Msg.Password)); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = h.DB.ExecContext(ctx, "INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)", sessionID, userID, expiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create session"))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": fmt.Sprintf("%d", userID),
		"jti": sessionID,
		"exp": expiresAt.Unix(),
	})
	signedToken, _ := token.SignedString([]byte(h.JWTSecret))

	return connect.NewResponse(&expensev1.LoginResponse{
		Message: "Login successful",
		User: &expensev1.User{
			Id:        userID,
			Username:  req.Msg.Username,
			CreatedAt: formatTime(createdAt),
		},
		Token:     signedToken,
	}), nil
}

func (h *AuthHandler) Logout(
	ctx context.Context,
	req *connect.Request[expensev1.LogoutRequest],
) (*connect.Response[expensev1.LogoutResponse], error) {
	sessionID, ok := middleware.SessionIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	_, err := h.DB.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", sessionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to logout"))
	}

	return connect.NewResponse(&expensev1.LogoutResponse{
		Message: "Logged out successfully",
	}), nil
}
