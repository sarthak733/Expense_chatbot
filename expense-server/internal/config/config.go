package config

import (
	"os"
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
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the environment. Call this once at
// startup in main.go — nothing else in the codebase should call
// os.Getenv directly, so every configurable value stays visible here.
func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		Env:             getEnv("APP_ENV", "development"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:sergtsop@localhost:5432/expense_tracker?host=/var/run/postgresql"),
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
