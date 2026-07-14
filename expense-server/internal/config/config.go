package config

import (
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration for the server.
// Values are read from environment variables with sane defaults,
// so the server runs locally with zero setup but stays configurable
// for staging/production without code changes.
type Config struct {
	Port            string
	Env             string
	DatabaseURL     string
	JWTSecret       string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func loadDotEnv() {
	content, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"' ` + "\r")
			os.Setenv(key, val)
		}
	}
}

// Load reads configuration from the environment. Call this once at
// startup in main.go — nothing else in the codebase should call
// os.Getenv directly, so every configurable value stays visible here.
//
// IMPORTANT: Set real values in a local .env file (never commit it).
// See .env.example for the list of variables.
func Load() Config {
	loadDotEnv()
	return Config{
		Port: getEnv("PORT", "8080"),
		Env:  getEnv("APP_ENV", "development"),

		// The fallback below uses an obviously-fake placeholder password.
		// The real DATABASE_URL must come from the .env file on your machine.
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:changeme@localhost:5432/expense_tracker"),

		// JWT_SECRET signs and verifies auth tokens.
		// Never hard-code a real secret here — always use the .env file.
		JWTSecret: getEnv("JWT_SECRET", ""),

		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
