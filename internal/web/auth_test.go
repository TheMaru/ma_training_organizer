package web_test

import (
	"database/sql"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/web"
)

const (
	testUsername = "trainer"
	testPassword = "correct-horse"
)

// newAuthTestServer starts an httptest server backed by a ready database that
// already holds one trainer (testUsername/testPassword), and returns a client
// whose cookie jar carries the session across requests. Redirects are not
// followed, so tests can assert on the 303/Location and HX-Redirect responses.
func newAuthTestServer(t *testing.T) (*httptest.Server, *http.Client, *sql.DB) {
	t.Helper()
	return newAuthTestServerIdle(t, time.Hour)
}

// newAuthTestServerIdle is newAuthTestServer with the idle timeout spelled out,
// for the tests that are about expiry itself.
func newAuthTestServerIdle(t *testing.T, idle time.Duration) (*httptest.Server, *http.Client, *sql.DB) {
	t.Helper()

	db := storetest.NewDB(t)
	addTrainer(t, db, testUsername)

	sessions := web.NewSessionManager(db, time.Hour, idle, false)
	srv, err := web.NewServer(db, sessions)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, newClient(t), db
}

// addTrainer creates a trainer who logs in with testPassword — the one the test
// server starts with, and any colleague a test needs beside them.
func addTrainer(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	hash, err := auth.Hash(testPassword)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if _, err := store.CreateTrainer(db, username, hash); err != nil {
		t.Fatalf("CreateTrainer %q: %v", username, err)
	}
}

// deleteTrainer removes a trainer's row. Raw SQL rather than a store operation
// because the subcommand that deletes an account is a later ticket
// (`.scratch/trainer-offboarding/issues/05`), and what the tests here need is an
// account that is gone, by whatever route.
func deleteTrainer(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	res, err := db.Exec(`DELETE FROM trainers WHERE username = ?`, username)
	if err != nil {
		t.Fatalf("delete trainer %q: %v", username, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("delete trainer %q affected %d rows, want 1", username, n)
	}
}

// newClient builds an HTTP client with its own cookie jar (so it carries one
// session) that does not follow redirects, letting tests assert on the 303 and
// HX-Redirect responses directly.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func login(t *testing.T, ts *httptest.Server, client *http.Client, username, password string) *http.Response {
	t.Helper()
	resp, err := client.PostForm(ts.URL+"/login", url.Values{
		"username": {username},
		"password": {password},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	return resp
}

func get(t *testing.T, ts *httptest.Server, client *http.Client, path string) *http.Response {
	t.Helper()
	resp, err := client.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// post submits a form to the given path, query string included (which
// client.PostForm cannot express), without following the redirect.
func post(t *testing.T, ts *httptest.Server, client *http.Client, path string, form url.Values) *http.Response {
	t.Helper()
	return postWith(t, ts, client, path, form, nil)
}

// postHTMX submits a form the way HTMX does, so a test can assert on the
// HX-Redirect answer rather than the 303.
func postHTMX(t *testing.T, ts *httptest.Server, client *http.Client, path string, form url.Values) *http.Response {
	t.Helper()
	return postWith(t, ts, client, path, form, http.Header{"HX-Request": {"true"}})
}

func postWith(t *testing.T, ts *httptest.Server, client *http.Client, path string, form url.Values, header http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// seeOtherTo asserts a plain 303 to the given target.
func seeOtherTo(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if got := resp.Header.Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// hxRedirectTo asserts the HTMX equivalent: 200 plus HX-Redirect.
func hxRedirectTo(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("HX-Redirect"); got != want {
		t.Errorf("HX-Redirect = %q, want %q", got, want)
	}
}

func TestUnauthenticatedAppRouteRedirectsToLogin(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := get(t, ts, client, "/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

func TestLoginWithCorrectCredentialsStartsSession(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := login(t, ts, client, testUsername, testPassword)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}

	// The session cookie now grants access to a gated route.
	home := get(t, ts, client, "/")
	defer home.Body.Close()
	if home.StatusCode != http.StatusOK {
		t.Errorf("GET / after login status = %d, want %d", home.StatusCode, http.StatusOK)
	}
}

func TestLoginWithWrongPasswordFails(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := login(t, ts, client, testUsername, "wrong")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// No session was started.
	home := get(t, ts, client, "/")
	defer home.Body.Close()
	if home.StatusCode != http.StatusSeeOther {
		t.Errorf("GET / after failed login status = %d, want %d (redirect)", home.StatusCode, http.StatusSeeOther)
	}
}

func TestLoginWithUnknownUserFails(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := login(t, ts, client, "nobody", testPassword)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/logout", nil)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("logout status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	home := get(t, ts, client, "/")
	defer home.Body.Close()
	if home.StatusCode != http.StatusSeeOther {
		t.Errorf("GET / after logout status = %d, want %d (redirect)", home.StatusCode, http.StatusSeeOther)
	}
}

func TestChangePasswordEndToEnd(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	const newPassword = "brand-new-secret"

	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/account/password", url.Values{
		"current": {testPassword},
		"new":     {newPassword},
		"confirm": {newPassword},
	})
	if err != nil {
		t.Fatalf("POST /account/password: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("change password status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	// The stored hash now verifies the new password and rejects the old one.
	tr, err := store.TrainerByUsername(db, testUsername)
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if ok, _ := auth.Verify(newPassword, tr.PasswordHash); !ok {
		t.Error("new password does not verify against stored hash")
	}
	if ok, _ := auth.Verify(testPassword, tr.PasswordHash); ok {
		t.Error("old password still verifies after change")
	}

	// A fresh client can log in with the new password but not the old one.
	fresh := newClient(t)
	if r := login(t, ts, fresh, testUsername, newPassword); r.StatusCode != http.StatusSeeOther {
		r.Body.Close()
		t.Errorf("login with new password status = %d, want %d", r.StatusCode, http.StatusSeeOther)
	} else {
		r.Body.Close()
	}
	if r := login(t, ts, newClient(t), testUsername, testPassword); r.StatusCode != http.StatusUnauthorized {
		r.Body.Close()
		t.Errorf("login with old password status = %d, want %d", r.StatusCode, http.StatusUnauthorized)
	} else {
		r.Body.Close()
	}
}

func TestChangePasswordWrongCurrentIsRejected(t *testing.T) {
	ts, client, db := newAuthTestServer(t)

	login(t, ts, client, testUsername, testPassword).Body.Close()
	before, _ := store.TrainerByUsername(db, testUsername)

	resp, err := client.PostForm(ts.URL+"/account/password", url.Values{
		"current": {"not-the-current"},
		"new":     {"another-secret"},
		"confirm": {"another-secret"},
	})
	if err != nil {
		t.Fatalf("POST /account/password: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	after, _ := store.TrainerByUsername(db, testUsername)
	if before.PasswordHash != after.PasswordHash {
		t.Error("password hash changed despite wrong current password")
	}
}

func TestChangePasswordMismatchIsRejected(t *testing.T) {
	ts, client, db := newAuthTestServer(t)

	login(t, ts, client, testUsername, testPassword).Body.Close()
	before, _ := store.TrainerByUsername(db, testUsername)

	resp, err := client.PostForm(ts.URL+"/account/password", url.Values{
		"current": {testPassword},
		"new":     {"secret-one"},
		"confirm": {"secret-two"},
	})
	if err != nil {
		t.Fatalf("POST /account/password: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	after, _ := store.TrainerByUsername(db, testUsername)
	if before.PasswordHash != after.PasswordHash {
		t.Error("password hash changed despite mismatched confirmation")
	}
}

func TestChangePasswordTooShortIsRejected(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	login(t, ts, client, testUsername, testPassword).Body.Close()

	short := strings.Repeat("a", auth.MinPasswordLength-1)
	resp, err := client.PostForm(ts.URL+"/account/password", url.Values{
		"current": {testPassword},
		"new":     {short},
		"confirm": {short},
	})
	if err != nil {
		t.Fatalf("POST /account/password: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPasswordPageRequiresAuth(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := get(t, ts, client, "/account/password")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}
