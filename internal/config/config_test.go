package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/config"
)

// clearEnv empties every variable Load reads, so a test starts from the
// defaults regardless of what the developer's own shell exports.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ORGANIZER_ADDR",
		"ORGANIZER_DB_PATH",
		"ORGANIZER_SESSION_LIFETIME",
		"ORGANIZER_SESSION_IDLE_TIMEOUT",
		"ORGANIZER_SECURE",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want \":8080\"", cfg.Addr)
	}
	if cfg.DBPath != "organizer.db" {
		t.Errorf("DBPath = %q, want \"organizer.db\"", cfg.DBPath)
	}
	if want := 30 * 24 * time.Hour; cfg.SessionLifetime != want {
		t.Errorf("SessionLifetime = %v, want %v", cfg.SessionLifetime, want)
	}
	if want := 7 * 24 * time.Hour; cfg.SessionIdleTimeout != want {
		t.Errorf("SessionIdleTimeout = %v, want %v", cfg.SessionIdleTimeout, want)
	}
	if cfg.Secure {
		t.Error("Secure = true, want false")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORGANIZER_ADDR", ":9999")
	t.Setenv("ORGANIZER_DB_PATH", "/data/organizer.db")
	t.Setenv("ORGANIZER_SESSION_LIFETIME", "12h")
	t.Setenv("ORGANIZER_SESSION_IDLE_TIMEOUT", "90m")
	t.Setenv("ORGANIZER_SECURE", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Addr != ":9999" {
		t.Errorf("Addr = %q, want \":9999\"", cfg.Addr)
	}
	if cfg.DBPath != "/data/organizer.db" {
		t.Errorf("DBPath = %q, want \"/data/organizer.db\"", cfg.DBPath)
	}
	if cfg.SessionLifetime != 12*time.Hour {
		t.Errorf("SessionLifetime = %v, want 12h", cfg.SessionLifetime)
	}
	if cfg.SessionIdleTimeout != 90*time.Minute {
		t.Errorf("SessionIdleTimeout = %v, want 1h30m", cfg.SessionIdleTimeout)
	}
	if !cfg.Secure {
		t.Error("Secure = false, want true")
	}
}

func TestStringNamesEveryField(t *testing.T) {
	cfg := config.Config{
		Addr:               ":9999",
		DBPath:             "/data/organizer.db",
		SessionLifetime:    30 * 24 * time.Hour,
		SessionIdleTimeout: 7 * 24 * time.Hour,
		Secure:             true,
	}

	got := cfg.String()

	for _, want := range []string{":9999", "/data/organizer.db", "720h", "168h", "secure=true"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

func TestLoadRejectsBadDurations(t *testing.T) {
	cases := map[string]string{
		"not a duration": "seven days",
		"bare number":    "7",
		"zero":           "0s",
		"negative":       "-24h",
	}
	for _, key := range []string{"ORGANIZER_SESSION_LIFETIME", "ORGANIZER_SESSION_IDLE_TIMEOUT"} {
		for name, value := range cases {
			t.Run(key+"/"+name, func(t *testing.T) {
				clearEnv(t)
				t.Setenv(key, value)

				_, err := config.Load()
				if err == nil {
					t.Fatalf("Load(%s=%q) = nil error, want a failure", key, value)
				}
				if !strings.Contains(err.Error(), key) {
					t.Errorf("error %q does not name %s", err, key)
				}
			})
		}
	}
}
