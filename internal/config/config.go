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
	// SessionLifetime is the absolute cap on a trainer session: how long it
	// stays valid counted from login, however active the trainer is.
	SessionLifetime time.Duration
	// SessionIdleTimeout expires a session that goes unused for this long,
	// inside SessionLifetime. It is what limits a stolen cookie's useful life
	// when nobody notices the theft, since a password change deliberately does
	// not revoke other sessions.
	SessionIdleTimeout time.Duration
	// Secure marks session cookies Secure (send over HTTPS only). Off in
	// local development, on in production behind Fly's TLS.
	Secure bool
}

// String renders the whole configuration on one line for the boot log. Config
// holds no secrets, so every field can be shown — and a wrong ORGANIZER_DB_PATH
// is then one visible line at startup instead of a server that looks correct
// while writing to the wrong database.
func (c Config) String() string {
	return fmt.Sprintf(
		"addr=%s db=%s session-lifetime=%s session-idle-timeout=%s secure=%t",
		c.Addr, c.DBPath, c.SessionLifetime, c.SessionIdleTimeout, c.Secure,
	)
}

// Load reads configuration from the environment, applying defaults suitable
// for local development.
func Load() (Config, error) {
	lifetime, err := envDuration("ORGANIZER_SESSION_LIFETIME", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	idle, err := envDuration("ORGANIZER_SESSION_IDLE_TIMEOUT", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:               env("ORGANIZER_ADDR", ":8080"),
		DBPath:             env("ORGANIZER_DB_PATH", "organizer.db"),
		SessionLifetime:    lifetime,
		SessionIdleTimeout: idle,
		Secure:             os.Getenv("ORGANIZER_SECURE") == "true",
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envDuration reads a Go duration string such as "168h" or "30m". A value that
// does not parse, or that is not positive, is a misconfiguration the operator
// has to see: both would otherwise land in the session manager as something
// plausible-looking and lock trainers out.
func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a duration (try \"168h\")", key, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: %s must be positive", key, d)
	}
	return d, nil
}
