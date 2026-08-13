package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// wantNoCookies asserts the client's jar holds nothing for the server. That is how
// a destroyed session differs from a merely refused one over HTTP: the browser is
// told to drop the cookie, so its next request carries no session at all.
func wantNoCookies(t *testing.T, ts *httptest.Server, client *http.Client) {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse %s: %v", ts.URL, err)
	}
	if cookies := client.Jar.Cookies(u); len(cookies) != 0 {
		t.Errorf("client still carries %v, want an empty jar", cookies)
	}
}

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

// A session holds a trainer id and nothing else, so nothing in it notices when the
// account behind it is gone. requireAuth is what notices, on every request.
func TestSessionDiesWithItsTrainer(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	deleteTrainer(t, db, testUsername)

	resp := get(t, ts, client, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status after the account was deleted = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
	wantNoCookies(t, ts, client)
}

// Deactivation is enforced per request, not only at login: a session started
// while the account was active must stop working the moment it is not, whether or
// not the operator's revocation reached it.
func TestSessionDiesWhenItsTrainerIsDeactivated(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	deactivate(t, db, testUsername)

	resp := get(t, ts, client, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status after the account was deactivated = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
	wantNoCookies(t, ts, client)
}

// The same guard against signing everybody out, for the deactivation case: one
// trainer's departure leaves the colleague still in the middle of a training.
func TestOnlyTheDeactivatedTrainersSessionDies(t *testing.T) {
	ts, mine, db := newAuthTestServer(t)
	login(t, ts, mine, testUsername, testPassword).Body.Close()

	const colleague = "grace"
	addTrainer(t, db, colleague)
	theirs := newClient(t)
	login(t, ts, theirs, colleague, testPassword).Body.Close()

	deactivate(t, db, testUsername)

	gone := get(t, ts, mine, "/")
	gone.Body.Close()
	if gone.StatusCode != http.StatusSeeOther {
		t.Errorf("deactivated trainer: GET / status = %d, want %d", gone.StatusCode, http.StatusSeeOther)
	}

	staying := get(t, ts, theirs, "/")
	staying.Body.Close()
	if staying.StatusCode != http.StatusOK {
		t.Errorf("colleague's GET / status = %d, want %d", staying.StatusCode, http.StatusOK)
	}
}

// The other half of the check above, and the reason it is a lookup per session
// rather than a sweep: an account going takes exactly one trainer's sessions with
// it. Without this the new check could pass by signing everybody out.
func TestOnlyTheDeletedTrainersSessionDies(t *testing.T) {
	ts, mine, db := newAuthTestServer(t)
	login(t, ts, mine, testUsername, testPassword).Body.Close()

	const colleague = "grace"
	addTrainer(t, db, colleague)
	theirs := newClient(t)
	login(t, ts, theirs, colleague, testPassword).Body.Close()

	deleteTrainer(t, db, testUsername)

	gone := get(t, ts, mine, "/")
	gone.Body.Close()
	if gone.StatusCode != http.StatusSeeOther {
		t.Errorf("deleted trainer: GET / status = %d, want %d", gone.StatusCode, http.StatusSeeOther)
	}

	staying := get(t, ts, theirs, "/")
	staying.Body.Close()
	if staying.StatusCode != http.StatusOK {
		t.Errorf("colleague's GET / status = %d, want %d", staying.StatusCode, http.StatusOK)
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
