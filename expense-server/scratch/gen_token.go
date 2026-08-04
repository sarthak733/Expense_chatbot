package scratch

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Usage: go run ./scratch/gen_token.go <user_id>
// Example: go run ./scratch/gen_token.go 1
func GenToken() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "ERROR: JWT_SECRET env var is not set")
		os.Exit(1)
	}

	userID := "1"
	if len(os.Args) > 1 {
		userID = os.Args[1]
		if _, err := strconv.Atoi(userID); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: user_id must be an integer, got: %s\n", userID)
			os.Exit(1)
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR signing token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(signed)
}
