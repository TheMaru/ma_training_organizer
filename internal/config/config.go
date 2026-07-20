// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime configuration. Every field is derived from an
// environment variable so the single binary is configured entirely by its
// environment (ADR-0002).
type Config struct {
	// Addr is the TCP address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// SessionLifetime is how long a trainer session stays valid.
	SessionLifetime time.Duration
	// Secure marks session cookies Secure (send over HTTPS only). Off in
	// local development, on in production behind Fly's TLS.
	Secure bool
}

// Load reads configuration from the environment, applying defaults suitable
// for local development.
func Load() (Config, error) {
	c := Config{
		Addr:            env("ORGANIZER_ADDR", ":8080"),
		DBPath:          env("ORGANIZER_DB_PATH", "organizer.db"),
		SessionLifetime: 30 * 24 * time.Hour,
		Secure:          os.Getenv("ORGANIZER_SECURE") == "true",
	}
	if c.Addr == "" {
		return Config{}, fmt.Errorf("ORGANIZER_ADDR must not be empty")
	}
	if c.DBPath == "" {
		return Config{}, fmt.Errorf("ORGANIZER_DB_PATH must not be empty")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
