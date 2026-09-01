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
	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
	"github.com/TheMaru/ma_training_organizer/internal/web"
)

const (
	testUsername = "trainer"
	testPassword = "correct-horse"
	// spareUsername is a second active trainer every fixture holds. An Offboarding
	// act refuses to take the club's last login away (trainer.ErrLastActiveTrainer),
	// so a server with one trainer on it is a state the Operator cannot reach — and
	// these tests set their state up through the acts the Operator invokes.
	spareUsername = "spare"
)

// newAuthTestServer starts an httptest server backed by a ready database that
// already holds two trainers (testUsername and spareUsername, both on
// testPassword), and returns a client whose cookie jar carries the session across
// requests. Redirects are not followed, so tests can assert on the 303/Location
// and HX-Redirect responses.
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
	addTrainer(t, db, spareUsername)

	sessions := session.ForServer(db, session.Policy{Lifetime: time.Hour, IdleTimeout: idle})
	srv, err := web.NewServer(db, sessions)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, newClient(t), db
}

// addTrainer provisions a trainer who logs in with testPassword — the one the
// test server starts with, and any colleague a test needs beside them.
//
// The helpers below go through internal/trainer, the acts the Operator invokes.
// What is under test in this package is the enforcement, not the acts — but the
// state it is verified against has to be the state those acts produce (see
// internal/trainer's package doc).
func addTrainer(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	if err := trainer.Provision(db, username, testPassword); err != nil {
		t.Fatalf("trainer.Provision %q: %v", username, err)
	}
}

// deleteTrainer removes a trainer's account outright, the erasure act.
func deleteTrainer(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	if err := trainer.Delete(db, username); err != nil {
		t.Fatalf("trainer.Delete %q: %v", username, err)
	}
}

// deactivate takes a trainer's access away, the ordinary Offboarding act — which
// revokes their Sessions as it goes, so a test asserting that enforcement notices
// on the next request signs in again afterwards.
func deactivate(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	if err := trainer.Deactivate(db, username); err != nil {
		t.Fatalf("trainer.Deactivate %q: %v", username, err)
	}
}

// trainerIDOf is the id a Session records, read while the account is still there —
// see session.Manager.RevokeAll for why the Session survives the row.
func trainerIDOf(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		t.Fatalf("TrainerByUsername %q: %v", username, err)
	}
	return tr.ID
}

// reactivate gives an account back, the way through the refusal.
func reactivate(t *testing.T, db *sql.DB, username string) {
	t.Helper()
	if err := trainer.Reactivate(db, username); err != nil {
		t.Fatalf("trainer.Reactivate %q: %v", username, err)
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

// A departed trainer's login has to fail like any other failed login: same
// status, same page, same message. A distinct answer would confirm the username
// exists, which is the disclosure handleLogin's decoy hash exists to prevent.
func TestDeactivatedTrainerLoginIsIndistinguishable(t *testing.T) {
	ts, _, db := newAuthTestServer(t)

	// The same username both times, so the two pages differ in nothing but the
	// reason they were rendered.
	wrong := login(t, ts, newClient(t), testUsername, "not-the-password")
	wrongStatus, wrongBody := wrong.StatusCode, readBody(t, wrong)

	deactivate(t, db, testUsername)

	client := newClient(t)
	refused := login(t, ts, client, testUsername, testPassword)
	refusedStatus, refusedBody := refused.StatusCode, readBody(t, refused)

	if refusedStatus != wrongStatus {
		t.Errorf("status = %d, want %d, the wrong-password answer", refusedStatus, wrongStatus)
	}
	if refusedBody != wrongBody {
		t.Errorf("body differs from the wrong-password answer:\n got: %s\nwant: %s", refusedBody, wrongBody)
	}

	home := get(t, ts, client, "/")
	defer home.Body.Close()
	if home.StatusCode != http.StatusSeeOther {
		t.Errorf("GET / after the refused login = %d, want %d (no session started)", home.StatusCode, http.StatusSeeOther)
	}
}

// The homecoming is only visible at a login: reactivation restores no password and
// starts no session, it just stops the refusal (ADR-0010). So this is where it is
// asserted — the act's own tests can see the state change but not what it buys.
func TestReactivatedTrainerLogsInWithTheSamePassword(t *testing.T) {
	ts, _, db := newAuthTestServer(t)
	deactivate(t, db, testUsername)
	refused := login(t, ts, newClient(t), testUsername, testPassword)
	refused.Body.Close()
	if refused.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login while deactivated = %d, want %d", refused.StatusCode, http.StatusUnauthorized)
	}

	reactivate(t, db, testUsername)

	client := newClient(t)
	resp := login(t, ts, client, testUsername, testPassword)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login after reactivation = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	// The session it started is honoured, so the account is back rather than merely
	// past the login form.
	home := get(t, ts, client, "/")
	defer home.Body.Close()
	if home.StatusCode != http.StatusOK {
		t.Errorf("GET / after reactivation = %d, want %d", home.StatusCode, http.StatusOK)
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
	ts, client, _ := newAuthTestServer(t)
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

	// A fresh client can log in with the new password but not the old one — which
	// is what the change is for, and says everything a look at the stored hash
	// would have said.
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
