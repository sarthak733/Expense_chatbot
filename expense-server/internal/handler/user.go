package handler

import (
	"expense-server/gen/expense/v1/expensev1connect"
)

// Compile-time proof that Handler satisfies the UserServiceHandler interface.
var _ expensev1connect.UserServiceHandler = (*Handler)(nil)
