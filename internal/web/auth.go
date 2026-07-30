package web

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// sessionKeyTrainerID is the session key under which the logged-in trainer's id
// is stored. A non-zero value is the sole signal that a request is authenticated.
const sessionKeyTrainerID = "trainerID"

// decoyHash is a valid argon2id hash of a throwaway value. handleLogin verifies
// against it when the username is unknown, so a login attempt performs the same
// argon2 work whether or not the account exists — closing the timing
// side-channel that would otherwise reveal which usernames are registered.
var decoyHash = mustDecoyHash()

func mustDecoyHash() string {
	h, err := auth.Hash("decoy: no trainer will ever have this password")
	if err != nil {
		panic("web: precompute decoy password hash: " + err.Error())
	}
	return h
}

// NewSessionManager builds an scs session manager backed by the SQLite sessions
// table (revocable server-side sessions, ADR-0002). secure marks the cookie
// Secure for production behind Fly's TLS; it is off in local development.
func NewSessionManager(db *sql.DB, lifetime time.Duration, secure bool) *scs.SessionManager {
	m := scs.New()
	m.Store = sqlite3store.New(db)
	m.Lifetime = lifetime
	m.Cookie.HttpOnly = true
	m.Cookie.SameSite = http.SameSiteLaxMode
	m.Cookie.Secure = secure
	return m
}

// requireAuth gates a route: requests without a logged-in trainer are redirected
// to the login page instead of reaching the handler.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.sessions.GetInt64(r.Context(), sessionKeyTrainerID) == 0 {
			redirect(w, r, "/login")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleLoginForm renders the login page. An already-authenticated trainer is
// bounced to the home page rather than shown the form again.
func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if s.sessions.GetInt64(r.Context(), sessionKeyTrainerID) != 0 {
		redirect(w, r, "/")
		return
	}
	s.renderLogin(w, r, http.StatusOK, "", "")
}

// handleLogin verifies submitted credentials and, on success, starts a session.
// Wrong password and unknown username are reported identically — and cost the
// same argon2 work (see decoyHash) — to avoid revealing which usernames exist.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	tr, err := store.TrainerByUsername(s.db, username)
	if err != nil {
		if errors.Is(err, store.ErrTrainerNotFound) {
			_, _ = auth.Verify(password, decoyHash) // equalise timing; result unused
			s.renderLogin(w, r, http.StatusUnauthorized, username, translate(r, "login.badCredentials"))
			return
		}
		serverError(w)
		return
	}

	ok, err := auth.Verify(password, tr.PasswordHash)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		s.renderLogin(w, r, http.StatusUnauthorized, username, translate(r, "login.badCredentials"))
		return
	}

	// Renew the token on privilege change to defend against session fixation.
	if err := s.sessions.RenewToken(r.Context()); err != nil {
		serverError(w)
		return
	}
	s.sessions.Put(r.Context(), sessionKeyTrainerID, tr.ID)
	redirect(w, r, "/")
}

// handleLogout destroys the session and returns the trainer to the login page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.Destroy(r.Context()); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, "/login")
}

// handlePasswordForm renders the self-service password-change page.
func (s *Server) handlePasswordForm(w http.ResponseWriter, r *http.Request) {
	s.renderPassword(w, r, http.StatusOK, "")
}

// handleChangePassword verifies the current password and applies the new one.
// New and confirmation must match and satisfy the shared password policy; the
// current password must be correct.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	current := r.PostFormValue("current")
	next := r.PostFormValue("new")
	confirm := r.PostFormValue("confirm")

	id := s.sessions.GetInt64(r.Context(), sessionKeyTrainerID)
	tr, err := store.TrainerByID(s.db, id)
	if err != nil {
		serverError(w)
		return
	}

	ok, err := auth.Verify(current, tr.PasswordHash)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		s.renderPassword(w, r, http.StatusUnauthorized, translate(r, "password.currentWrong"))
		return
	}
	if next != confirm {
		s.renderPassword(w, r, http.StatusBadRequest, translate(r, "password.mismatch"))
		return
	}
	if err := auth.ValidatePassword(next); err != nil {
		s.renderPassword(w, r, http.StatusBadRequest, translate(r, "password.tooShort"))
		return
	}

	hash, err := auth.Hash(next)
	if err != nil {
		serverError(w)
		return
	}
	if err := store.UpdateTrainerPassword(s.db, tr.ID, hash); err != nil {
		serverError(w)
		return
	}

	// A password change is a privilege change, so rotate this session's token as
	// defence in depth against fixation. Note scs renews only the current
	// session; sessions established on other devices are unaffected.
	if err := s.sessions.RenewToken(r.Context()); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, "/")
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, status int, username, errMsg string) {
	s.tmpl.render(w, r, status, "login.html", map[string]any{
		"Authenticated": false,
		"Username":      username,
		"Error":         errMsg,
	})
}

func (s *Server) renderPassword(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	s.tmpl.render(w, r, status, "password.html", map[string]any{
		"Authenticated": true,
		"Error":         errMsg,
	})
}

// serverError sends a generic 500 without leaking internal detail to the client.
// The panic-free error paths in the auth handlers all funnel through here.
func serverError(w http.ResponseWriter) {
	http.Error(w, "internal error", http.StatusInternalServerError)
}
