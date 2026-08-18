package session

import "context"

// RevokeAll ends every Session trainerID holds, sparing none, and reports how
// many that was.
//
// It walks the entire session store. The Trainer id lives inside each Session's
// encoded values, which SQL cannot reach into, so there is no narrower query to
// run: the cost is a property of this interface (ADR-0002 chose revocable
// server-side sessions) and not of how it happens to be implemented today.
func (m *Manager) RevokeAll(ctx context.Context, trainerID int64) (int, error) {
	return m.eachSessionOf(ctx, trainerID, "", m.scs.Destroy)
}

// RevokeOthers ends every Session this request's Trainer holds except the one the
// request arrived on. It takes no token, because the request already is the
// Session to keep.
//
// It walks the whole session store, as RevokeAll does and for the same reason.
func (m *Manager) RevokeOthers(ctx context.Context) error {
	_, err := m.eachSessionOf(ctx, m.TrainerID(ctx), m.scs.Token(ctx), m.scs.Destroy)
	return err
}

// Count is how many Sessions trainerID holds — what lets a caller see a
// revocation as such, from either side of it. It walks the store for the same
// reason RevokeAll does.
func (m *Manager) Count(ctx context.Context, trainerID int64) (int, error) {
	return m.eachSessionOf(ctx, trainerID, "", nil)
}

// eachSessionOf runs act for every stored Session that holds trainerID, except
// the one whose token is except, and reports how many it ran for. A nil act
// counts them and touches nothing.
func (m *Manager) eachSessionOf(ctx context.Context, trainerID int64, except string, act func(context.Context) error) (int, error) {
	n := 0
	err := m.scs.Iterate(ctx, func(ctx context.Context) error {
		if !m.holds(ctx, trainerID, except) {
			return nil
		}
		if act != nil {
			if err := act(ctx); err != nil {
				return err
			}
		}
		n++
		return nil
	})
	return n, err
}

// holds reports whether the Session in ctx is one of trainerID's, other than the
// one the token except names. An empty except excludes nothing.
//
// Trainer zero is nobody, and it is also what every Session that has never been
// signed into reads as — so it holds nothing. Without that, one caller passing a
// missing id would sweep up every visitor who has only been shown a message.
func (m *Manager) holds(ctx context.Context, trainerID int64, except string) bool {
	if trainerID == 0 || m.scs.GetInt64(ctx, keyTrainerID) != trainerID {
		return false
	}
	return except == "" || m.scs.Token(ctx) != except
}
