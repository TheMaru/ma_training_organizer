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

	db := storetest.NewDB(t)
	if err := createTrainer(db, username, password); err != nil {
		t.Fatalf("createTrainer: %v", err)
	}

	srv, err := web.NewServer(db, web.NewSessionManager(db, time.Hour, time.Hour, false))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	var clients []*http.Client
	for range 2 {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookiejar: %v", err)
		}
		c := &http.Client{
			Jar:           jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		resp, err := c.PostForm(ts.URL+"/login", url.Values{
			"username": {username},
			"password": {password},
		})
		if err != nil {
			t.Fatalf("POST /login: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		clients = append(clients, c)
	}
	return db, ts, clients
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
