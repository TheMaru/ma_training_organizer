package web_test

import (
	"net/http"
	"testing"
	"time"
)

// TestSessionExpiresWhenIdle drives the idle timeout with a millisecond value
// rather than the configured seven days: what is under test is that the manager
// applies an idle timeout at all, which no unit of time changes.
func TestSessionExpiresWhenIdle(t *testing.T) {
	const idle = 50 * time.Millisecond
	ts, client, _ := newAuthTestServerIdle(t, idle)

	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := get(t, ts, client, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status right after login = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	time.Sleep(2 * idle)

	resp = get(t, ts, client, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status after idling = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

// TestActivityKeepsSessionAlive is the other half: an idle timeout shorter than
// the test's own duration must not log out a trainer who keeps using the app.
func TestActivityKeepsSessionAlive(t *testing.T) {
	const idle = 100 * time.Millisecond
	ts, client, _ := newAuthTestServerIdle(t, idle)

	login(t, ts, client, testUsername, testPassword).Body.Close()

	for range 4 {
		time.Sleep(idle / 2)
		resp := get(t, ts, client, "/")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status while active = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	}
}
