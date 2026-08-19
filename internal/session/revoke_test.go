package session_test

import (
	"context"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/session/sessiontest"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// The two trainers these tests share. Ids rather than accounts: a Session records
// whose it is and nothing more, so this module never looks a Trainer up.
const (
	ada   = int64(1)
	grace = int64(2)
)

// newManager returns a Manager over a throwaway database. The scaffolding for
// starting a Session through the middleware lives in sessiontest, because
// internal/trainer's tests need Sessions too and there is one way to make one.
func newManager(t *testing.T) *session.Manager {
	t.Helper()
	return sessiontest.NewManager(t, storetest.NewDB(t))
}

// The Operator's verb: a lost phone means every device the Trainer holds, the one
// they are reading this on included. The colleague at the same club keeps theirs.
func TestRevokeAllEndsEverySessionTheTrainerHolds(t *testing.T) {
	m := newManager(t)
	phone := sessiontest.SignIn(t, m, ada)
	laptop := sessiontest.SignIn(t, m, ada)
	colleague := sessiontest.SignIn(t, m, grace)

	revoked, err := m.RevokeAll(context.Background(), ada)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}

	if revoked != 2 {
		t.Errorf("revoked = %d, want 2", revoked)
	}
	if got := phone.SignedInAs(t, m); got != 0 {
		t.Errorf("phone still signed in as %d", got)
	}
	if got := laptop.SignedInAs(t, m); got != 0 {
		t.Errorf("laptop still signed in as %d", got)
	}
	if got := colleague.SignedInAs(t, m); got != grace {
		t.Errorf("colleague signed in as %d, want %d", got, grace)
	}
}

// The Trainer's own verb, and the difference between the two: "the others" is
// every Session but the one the request asking arrived on. No token is passed,
// because the request already is that Session.
func TestRevokeOthersKeepsTheSessionItWasCalledFrom(t *testing.T) {
	m := newManager(t)
	phone := sessiontest.SignIn(t, m, ada)
	laptop := sessiontest.SignIn(t, m, ada)

	laptop.Do(t, m, func(ctx context.Context) {
		if err := m.RevokeOthers(ctx); err != nil {
			t.Fatalf("RevokeOthers: %v", err)
		}
	})

	if got := laptop.SignedInAs(t, m); got != ada {
		t.Errorf("the revoking device is signed in as %d, want %d", got, ada)
	}
	if got := phone.SignedInAs(t, m); got != 0 {
		t.Errorf("the other device is still signed in as %d", got)
	}
}

// Revocation is per Trainer whichever verb takes it, so the colleague in the
// middle of a training is not swept up by somebody else's control.
func TestRevokeOthersSparesTheOtherTrainer(t *testing.T) {
	m := newManager(t)
	laptop := sessiontest.SignIn(t, m, ada)
	colleague := sessiontest.SignIn(t, m, grace)

	laptop.Do(t, m, func(ctx context.Context) {
		if err := m.RevokeOthers(ctx); err != nil {
			t.Fatalf("RevokeOthers: %v", err)
		}
	})

	if got := colleague.SignedInAs(t, m); got != grace {
		t.Errorf("colleague signed in as %d, want %d", got, grace)
	}
}

// The question a caller asks to see a revocation as such — including the Operator's
// CLI, which reports how many Sessions it ended.
func TestCountAnswersHowManySessionsATrainerHolds(t *testing.T) {
	m := newManager(t)
	sessiontest.SignIn(t, m, ada)
	sessiontest.SignIn(t, m, ada)
	sessiontest.SignIn(t, m, grace)

	if got := sessiontest.Count(t, m, ada); got != 2 {
		t.Errorf("ada holds %d sessions, want 2", got)
	}
	if got := sessiontest.Count(t, m, grace); got != 1 {
		t.Errorf("grace holds %d sessions, want 1", got)
	}
	if got := sessiontest.Count(t, m, int64(99)); got != 0 {
		t.Errorf("a trainer who never signed in holds %d sessions, want 0", got)
	}
}

// Zero is not a Trainer, it is the absence of one — and it is what every Session
// nobody has signed into reads as. So the verbs must refuse it rather than treat
// the anonymous Sessions as one Trainer's: a visitor who has only been shown a
// message would otherwise lose it whenever anybody revoked.
func TestNobodyHoldsTheAnonymousSessions(t *testing.T) {
	m := newManager(t)
	visitor := (&sessiontest.Device{}).Do(t, m, func(ctx context.Context) {
		m.Put(ctx, "notice", "hello")
	})

	revoked, err := m.RevokeAll(context.Background(), 0)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}

	if revoked != 0 {
		t.Errorf("revoked = %d, want 0", revoked)
	}
	if got := sessiontest.Count(t, m, 0); got != 0 {
		t.Errorf("Count(0) = %d, want 0", got)
	}
	var notice string
	visitor.Do(t, m, func(ctx context.Context) { notice = m.Pop(ctx, "notice") })
	if notice != "hello" {
		t.Errorf("the visitor's pending value = %q, want %q", notice, "hello")
	}
}
