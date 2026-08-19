// Package sessiontest starts real Sessions for tests. A Session exists only once
// the middleware has written one, so a test that needs a Trainer signed in on a
// device has to make a request — and what that takes is scaffolding rather than
// anything a test wants to say. It lives in a package of its own so that
// internal/session's own tests and internal/trainer's share one answer to "what
// does a signed-in device look like?", the way storetest is one answer to what a
// test database looks like.
//
// The Manager here is the server's, because that is the only one that can start a
// Session. A command's Manager (session.ForCommand) reaches the same stored
// Sessions, which is what lets a test hold one while the act under test builds the
// other.
package sessiontest

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/session"
)

// NewManager returns a Manager over db, as a server runs it. The timeouts are
// long enough that nothing expires while a test runs: expiry has its own tests
// (internal/web), and a Session that ended by itself here would read as a
// revocation that never happened.
func NewManager(t *testing.T, db *sql.DB) *session.Manager {
	t.Helper()
	return session.ForServer(db, session.Policy{
		Lifetime:    time.Hour,
		IdleTimeout: time.Hour,
	})
}

// Device stands for one device: the Manager it talks to and the cookie it carries,
// which is the whole of what a device holds. NewDevice makes one that has never
// been signed in — a visitor.
type Device struct {
	sessions *session.Manager
	cookie   *http.Cookie
}

// NewDevice is a device carrying nothing yet, for the tests about what a visitor
// keeps across a revocation.
func NewDevice(m *session.Manager) *Device {
	return &Device{sessions: m}
}

// SignIn starts a Session for trainerID and returns the Device holding it. It
// goes through the middleware, so what a test then revokes is a Session made the
// way the app makes one.
func SignIn(t *testing.T, m *session.Manager, trainerID int64) *Device {
	t.Helper()
	return NewDevice(m).Do(t, func(ctx context.Context) {
		m.SetTrainerID(ctx, trainerID)
	})
}

// Do makes one request from this Device, running fn inside the session
// middleware, and keeps whatever cookie comes back.
func (d *Device) Do(t *testing.T, fn func(ctx context.Context)) *Device {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if d.cookie != nil {
		r.AddCookie(d.cookie)
	}
	rec := httptest.NewRecorder()
	d.sessions.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		fn(r.Context())
	})).ServeHTTP(rec, r)
	for _, c := range rec.Result().Cookies() {
		d.cookie = c
	}
	return d
}

// Cookie is what this Device carries: the token naming its Session, ready to be
// handed to an http.Client whose requests should arrive holding it.
func (d *Device) Cookie() *http.Cookie {
	return d.cookie
}

// SignedInAs is whose Session this Device still carries, as the app asks on every
// request. Zero is what a revoked Device gets: its cookie names a Session that is
// no longer there.
func (d *Device) SignedInAs(t *testing.T) int64 {
	t.Helper()
	var id int64
	d.Do(t, func(ctx context.Context) { id = d.sessions.TrainerID(ctx) })
	return id
}

// Count is how many Sessions trainerID holds, asked of the module that owns them
// and fatal if the question itself fails.
func Count(t *testing.T, m *session.Manager, trainerID int64) int {
	t.Helper()
	n, err := m.Count(context.Background(), trainerID)
	if err != nil {
		t.Fatalf("Count(%d): %v", trainerID, err)
	}
	return n
}
