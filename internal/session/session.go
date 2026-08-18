// Package session owns a Trainer's Session: the running sign-in on one device
// (CONTEXT.md). It is the only package that imports scs, the session library
// ADR-0002 chose, so every other package asks it for a Trainer, a flash value or
// a revocation instead of holding a session store of its own.
//
// That the Trainer's identity lives here is what makes the rest possible: it is
// stored inside each Session's encoded values, so a package that cannot read
// that key cannot answer "whose Session is this?" — and revocation is exactly
// that question asked of every stored Session.
package session

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

// keyTrainerID is the stored key under which a Session records whose it is. A
// non-zero value is the sole signal that a request is authenticated.
//
// It is a data format rather than an identifier: every Session already in a
// database spells it this way, so renaming it would sign every Trainer out and
// buy nothing.
const keyTrainerID = "trainerID"

// callerKeyPrefix namespaces the keys Put and Pop take. A caller names its own
// keys, so without the prefix a caller that happened to pick keyTrainerID's
// spelling could overwrite whose Session it is.
const callerKeyPrefix = "caller:"

// Policy is what a server states about its Sessions. It is passed in rather than
// read from the environment, so this module does not depend on where
// configuration comes from — main builds it from internal/config.
type Policy struct {
	// Lifetime is the absolute cap on a Session, counted from sign-in however
	// active the Trainer is.
	Lifetime time.Duration
	// IdleTimeout ends a Session that goes unused for this long, inside Lifetime.
	IdleTimeout time.Duration
	// SecureCookie sends the session cookie over HTTPS only: on in production
	// behind Fly's TLS, off in local development.
	SecureCookie bool
}

// Manager holds Sessions and answers everything asked about them. It hides scs
// rather than handing it out, so no caller has to obtain a session manager to
// ask a question about a Trainer's Sessions.
type Manager struct {
	scs *scs.SessionManager
}

// ForServer builds the Manager an HTTP server runs on: the SQLite-backed store
// (ADR-0002), the cookie policy, and scs's background cleanup of expired rows.
func ForServer(db *sql.DB, p Policy) *Manager {
	m := scs.New()
	m.Store = sqlite3store.New(db)
	m.Lifetime = p.Lifetime
	m.IdleTimeout = p.IdleTimeout
	m.Cookie.HttpOnly = true
	m.Cookie.SameSite = http.SameSiteLaxMode
	m.Cookie.Secure = p.SecureCookie
	return &Manager{scs: m}
}

// ForCommand builds the Manager a one-shot command works through: the stored
// Sessions and nothing else. A command serves no request, so it has no cookie to
// set and no lifetime of its own to state, and it exits far too quickly for
// background cleanup to be anything but a goroutine left behind.
func ForCommand(db *sql.DB) *Manager {
	m := scs.New()
	m.Store = sqlite3store.NewWithCleanupInterval(db, 0)
	return &Manager{scs: m}
}

// Middleware loads the request's Session before the handler runs and saves it
// afterwards. Everything below reads and writes the Session through the request
// context it puts there, so a handler outside this middleware sees no Session at
// all.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return m.scs.LoadAndSave(next)
}

// TrainerID is whose Session this request carries. Zero means nobody is signed
// in, which is the only signal there is: a Session holds an id and nothing else,
// so whether that Trainer may still use the app is a question for the caller.
func (m *Manager) TrainerID(ctx context.Context) int64 {
	return m.scs.GetInt64(ctx, keyTrainerID)
}

// SetTrainerID records whose Session this is, which is what signing in amounts
// to. Renew belongs immediately before it, the sign-in being a privilege change.
func (m *Manager) SetTrainerID(ctx context.Context, id int64) {
	m.scs.Put(ctx, keyTrainerID, id)
}

// Renew issues this Session a fresh token and keeps what it holds. It is the
// defence against session fixation: a token planted before a privilege change
// does not survive it.
func (m *Manager) Renew(ctx context.Context) error {
	return m.scs.RenewToken(ctx)
}

// Destroy ends this request's Session and tells the browser to drop its cookie,
// so the next request is a first visit rather than the same refusal again.
func (m *Manager) Destroy(ctx context.Context) error {
	return m.scs.Destroy(ctx)
}

// Put leaves value in this Session under the caller's key until Pop takes it.
// The key is the caller's to name: it is stored in a space of its own, so it
// cannot collide with what this module keeps there.
func (m *Manager) Put(ctx context.Context, key, value string) {
	m.scs.Put(ctx, callerKeyPrefix+key, value)
}

// Pop takes what Put left under key, and empties it. An empty string means there
// was nothing there — a value waits until it is read, so a Trainer who never
// reaches the page that reads it keeps it until they do.
func (m *Manager) Pop(ctx context.Context, key string) string {
	return m.scs.PopString(ctx, callerKeyPrefix+key)
}

// RevokeSessions destroys every stored Session belonging to trainerID. The
// Session whose token is exceptToken survives; an empty exceptToken spares
// nothing. That one argument is the whole difference between the two callers:
// the Trainer's own "sign out other devices" control passes the token it was
// clicked from, the Operator's CLI passes none.
//
// It walks the entire session store because the Trainer id lives inside each
// Session's encoded values, which SQL cannot reach into — so there is no
// narrower query to run.
func (m *Manager) RevokeSessions(ctx context.Context, trainerID int64, exceptToken string) error {
	return m.scs.Iterate(ctx, func(ctx context.Context) error {
		if m.scs.GetInt64(ctx, keyTrainerID) != trainerID {
			return nil
		}
		if exceptToken != "" && m.scs.Token(ctx) == exceptToken {
			return nil
		}
		return m.scs.Destroy(ctx)
	})
}

// Token is this request's session token, the argument RevokeSessions takes to
// spare the Session it was called from.
func (m *Manager) Token(ctx context.Context) string {
	return m.scs.Token(ctx)
}
