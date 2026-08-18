package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// The two trainers these tests share. Ids rather than accounts: a Session records
// whose it is and nothing more, so this module never looks a Trainer up.
const (
	ada   = int64(1)
	grace = int64(2)
)

// newManager returns a Manager over a throwaway database, as a server runs it.
// The server's constructor rather than the command's, because a Session can only
// be started through the middleware — which is also how the app makes one.
func newManager(t *testing.T) *session.Manager {
	t.Helper()
	return session.ForServer(storetest.NewDB(t), session.Policy{
		Lifetime:    time.Hour,
		IdleTimeout: time.Hour,
	})
}

// device stands for one device holding one Session: the cookie is the whole of
// what a device carries.
type device struct {
	cookie *http.Cookie
}

// signIn starts a Session for trainerID and returns the device holding it. It
// goes through the middleware, so what these tests revoke is a Session made the
// way the app makes one.
func signIn(t *testing.T, m *session.Manager, trainerID int64) *device {
	t.Helper()
	return (&device{}).do(t, m, func(ctx context.Context) {
		m.SetTrainerID(ctx, trainerID)
	})
}

// do makes one request from this device, running fn inside the session
// middleware, and keeps whatever cookie comes back.
func (d *device) do(t *testing.T, m *session.Manager, fn func(ctx context.Context)) *device {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if d.cookie != nil {
		r.AddCookie(d.cookie)
	}
	rec := httptest.NewRecorder()
	m.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		fn(r.Context())
	})).ServeHTTP(rec, r)
	for _, c := range rec.Result().Cookies() {
		d.cookie = c
	}
	return d
}

// signedInAs is whose Session this device still carries, as the app asks on every
// request. Zero is what a revoked device gets: its cookie names a Session that is
// no longer there.
func (d *device) signedInAs(t *testing.T, m *session.Manager) int64 {
	t.Helper()
	var id int64
	d.do(t, m, func(ctx context.Context) { id = m.TrainerID(ctx) })
	return id
}

func count(t *testing.T, m *session.Manager, trainerID int64) int {
	t.Helper()
	n, err := m.Count(context.Background(), trainerID)
	if err != nil {
		t.Fatalf("Count(%d): %v", trainerID, err)
	}
	return n
}

// The Operator's verb: a lost phone means every device the Trainer holds, the one
// they are reading this on included. The colleague at the same club keeps theirs.
func TestRevokeAllEndsEverySessionTheTrainerHolds(t *testing.T) {
	m := newManager(t)
	phone := signIn(t, m, ada)
	laptop := signIn(t, m, ada)
	colleague := signIn(t, m, grace)

	revoked, err := m.RevokeAll(context.Background(), ada)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}

	if revoked != 2 {
		t.Errorf("revoked = %d, want 2", revoked)
	}
	if got := phone.signedInAs(t, m); got != 0 {
		t.Errorf("phone still signed in as %d", got)
	}
	if got := laptop.signedInAs(t, m); got != 0 {
		t.Errorf("laptop still signed in as %d", got)
	}
	if got := colleague.signedInAs(t, m); got != grace {
		t.Errorf("colleague signed in as %d, want %d", got, grace)
	}
}

// The Trainer's own verb, and the difference between the two: "the others" is
// every Session but the one the request asking arrived on. No token is passed,
// because the request already is that Session.
func TestRevokeOthersKeepsTheSessionItWasCalledFrom(t *testing.T) {
	m := newManager(t)
	phone := signIn(t, m, ada)
	laptop := signIn(t, m, ada)

	laptop.do(t, m, func(ctx context.Context) {
		if err := m.RevokeOthers(ctx); err != nil {
			t.Fatalf("RevokeOthers: %v", err)
		}
	})

	if got := laptop.signedInAs(t, m); got != ada {
		t.Errorf("the revoking device is signed in as %d, want %d", got, ada)
	}
	if got := phone.signedInAs(t, m); got != 0 {
		t.Errorf("the other device is still signed in as %d", got)
	}
}

// Revocation is per Trainer whichever verb takes it, so the colleague in the
// middle of a training is not swept up by somebody else's control.
func TestRevokeOthersSparesTheOtherTrainer(t *testing.T) {
	m := newManager(t)
	laptop := signIn(t, m, ada)
	colleague := signIn(t, m, grace)

	laptop.do(t, m, func(ctx context.Context) {
		if err := m.RevokeOthers(ctx); err != nil {
			t.Fatalf("RevokeOthers: %v", err)
		}
	})

	if got := colleague.signedInAs(t, m); got != grace {
		t.Errorf("colleague signed in as %d, want %d", got, grace)
	}
}

// The question a caller asks to see a revocation as such — including the Operator's
// CLI, which reports how many Sessions it ended.
func TestCountAnswersHowManySessionsATrainerHolds(t *testing.T) {
	m := newManager(t)
	signIn(t, m, ada)
	signIn(t, m, ada)
	signIn(t, m, grace)

	if got := count(t, m, ada); got != 2 {
		t.Errorf("ada holds %d sessions, want 2", got)
	}
	if got := count(t, m, grace); got != 1 {
		t.Errorf("grace holds %d sessions, want 1", got)
	}
	if got := count(t, m, int64(99)); got != 0 {
		t.Errorf("a trainer who never signed in holds %d sessions, want 0", got)
	}
}

// Zero is not a Trainer, it is the absence of one — and it is what every Session
// nobody has signed into reads as. So the verbs must refuse it rather than treat
// the anonymous Sessions as one Trainer's: a visitor who has only been shown a
// message would otherwise lose it whenever anybody revoked.
func TestNobodyHoldsTheAnonymousSessions(t *testing.T) {
	m := newManager(t)
	visitor := (&device{}).do(t, m, func(ctx context.Context) {
		m.Put(ctx, "notice", "hello")
	})

	revoked, err := m.RevokeAll(context.Background(), 0)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}

	if revoked != 0 {
		t.Errorf("revoked = %d, want 0", revoked)
	}
	if got := count(t, m, 0); got != 0 {
		t.Errorf("Count(0) = %d, want 0", got)
	}
	var notice string
	visitor.do(t, m, func(ctx context.Context) { notice = m.Pop(ctx, "notice") })
	if notice != "hello" {
		t.Errorf("the visitor's pending value = %q, want %q", notice, "hello")
	}
}
