package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/web"
)

// signedIn starts the app on a throwaway database and returns two clients logged
// in as the same trainer, the operator's situation: somebody is on their phone
// and their laptop and neither is reachable from the CLI.
func signedIn(t *testing.T, username, password string) (*sql.DB, *httptest.Server, []*http.Client) {
	t.Helper()

	db, ts := startApp(t)
	if err := createTrainer(db, username, password); err != nil {
		t.Fatalf("createTrainer: %v", err)
	}
	return db, ts, []*http.Client{
		signIn(t, ts, username, password),
		signIn(t, ts, username, password),
	}
}

// startApp serves the app over a throwaway database and returns both. The server
// keeps its own session manager, so a test that drives a subcommand's core
// against the same database exercises the real arrangement: a second process
// reaching the same store.
func startApp(t *testing.T) (*sql.DB, *httptest.Server) {
	t.Helper()

	db := storetest.NewDB(t)
	srv, err := web.NewServer(db, web.NewSessionManager(db, time.Hour, time.Hour, false))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return db, ts
}

// signIn logs a fresh client in and fails the test unless it worked. The returned
// client's jar carries that session, so it stands for one device.
func signIn(t *testing.T, ts *httptest.Server, username, password string) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	c := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if got := postLogin(t, c, ts, username, password); got != http.StatusSeeOther {
		t.Fatalf("login status for %q = %d, want %d", username, got, http.StatusSeeOther)
	}
	return c
}

// loginStatus attempts a login from a client that has no session yet and reports
// the status, for the tests where being refused is the expected outcome.
func loginStatus(t *testing.T, ts *httptest.Server, username, password string) int {
	t.Helper()
	return postLogin(t, &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, ts, username, password)
}

func postLogin(t *testing.T, c *http.Client, ts *httptest.Server, username, password string) int {
	t.Helper()
	resp, err := c.PostForm(ts.URL+"/login", url.Values{
		"username": {username},
		"password": {password},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// The operator's path spares nothing — including the session that, on the
// self-service control, would have been the caller's own.
func TestRevokeSessionsEndsEveryDevice(t *testing.T) {
	db, ts, clients := signedIn(t, "ada", "correct-horse")

	// The running server keeps its own session manager, so this exercises the real
	// arrangement: a second process reaching the same store.
	if err := revokeSessions(db, "ada"); err != nil {
		t.Fatalf("revokeSessions: %v", err)
	}

	for i, c := range clients {
		resp, err := c.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("client %d: GET / status = %d, want %d", i, resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Errorf("client %d: Location = %q, want %q", i, loc, "/login")
		}
	}
}

func TestRevokeSessionsUnknownTrainer(t *testing.T) {
	db := storetest.NewDB(t)

	err := revokeSessions(db, "ghost")
	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}
