package web

import (
	"errors"
	"net/http"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// sessionKeyNotice carries a one-off message over the redirect that follows a
// mutation, so the page it lands on can report what happened. It holds a catalog
// key rather than a sentence, so the message is translated when it is read: a
// trainer who switches language in between gets the notice in the language they
// are now reading. The page that shows it takes it, but a trainer who never
// arrives there keeps it until they do.
const sessionKeyNotice = "notice"

const (
	passwordPath = "/account/password"
	revokePath   = "/account/sessions/revoke"
)

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

// requireAuth gates a route: requests without a logged-in trainer are redirected
// to the login page instead of reaching the handler.
//
// The id in the session is not taken as proof that the account may still be used,
// so the trainer is loaded on every request and their state read. Without that, a
// session outlives the account it belongs to — deleting or deactivating a trainer
// would leave their browser with full access until the session happened to
// expire, and revocation is a separate act that cannot be relied on to have
// happened (ADR-0010).
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := s.sessions.TrainerID(r.Context())
		if id == 0 {
			redirect(w, r, "/login")
			return
		}
		mayContinue, err := s.trainerMayUseTheApp(id)
		if err != nil {
			serverError(w)
			return
		}
		if mayContinue {
			next.ServeHTTP(w, r)
			return
		}
		// The session names nobody who may log in, so it is worth nothing:
		// destroying it makes the next request a first visit rather than this same
		// check again.
		if err := s.sessions.Destroy(r.Context()); err != nil {
			serverError(w)
			return
		}
		redirect(w, r, "/login")
	})
}

// trainerMayUseTheApp answers the question requireAuth has about the id in a
// session. An account that is gone and one that is deactivated are both a plain
// "no" rather than an error: only a database that could not answer is.
func (s *Server) trainerMayUseTheApp(id int64) (bool, error) {
	tr, err := store.TrainerByID(s.db, id)
	switch {
	case errors.Is(err, store.ErrTrainerNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return !tr.Deactivated(), nil
}

// handleLoginForm renders the login page. An already-authenticated trainer is
// bounced to the home page rather than shown the form again.
func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if s.sessions.TrainerID(r.Context()) != 0 {
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
	// A deactivated account is refused exactly as a wrong password is (ADR-0010):
	// naming the state would confirm the username exists, which is what the decoy
	// hash above exists to prevent. The state is read only after auth.Verify has
	// run, so the refusal costs the same argon2 work as any other failed login and
	// does not become visible in the response time either.
	if !ok || tr.Deactivated() {
		s.renderLogin(w, r, http.StatusUnauthorized, username, translate(r, "login.badCredentials"))
		return
	}

	// Renew the token on privilege change to defend against session fixation.
	if err := s.sessions.Renew(r.Context()); err != nil {
		serverError(w)
		return
	}
	s.sessions.SetTrainerID(r.Context(), tr.ID)
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

	id := s.sessions.TrainerID(r.Context())
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
	// defence in depth against fixation. Renew touches only the current session,
	// and leaving the others alone is the decision here, not an oversight: revoking
	// them is a separate, explicit action, so neither has to guess at the other's
	// intent. The two ways to take it are handleRevokeSessions, in the account
	// area beneath this very form, and the CLI's revoke-sessions.
	if err := s.sessions.Renew(r.Context()); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, "/")
}

// handleRevokeSessions is the self-service half of revocation: it ends the
// trainer's sessions everywhere but here. The session it was clicked from has
// just been authenticated, so it is the one kept.
func (s *Server) handleRevokeSessions(w http.ResponseWriter, r *http.Request) {
	id := s.sessions.TrainerID(r.Context())
	if err := s.sessions.RevokeSessions(r.Context(), id, s.sessions.Token(r.Context())); err != nil {
		serverError(w)
		return
	}
	// Nothing visible changes on this device, so without a message the trainer has
	// no way to tell the action from a no-op.
	s.sessions.Put(r.Context(), sessionKeyNotice, "sessions.revoked")
	redirect(w, r, passwordPath)
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
		"Notice":        s.popNotice(r),
		"Error":         errMsg,
	})
}

// popNotice takes the pending notice, if there is one, and renders it in the
// request's language. An empty string means there is nothing to report.
func (s *Server) popNotice(r *http.Request) string {
	key := s.sessions.Pop(r.Context(), sessionKeyNotice)
	if key == "" {
		return ""
	}
	return translate(r, key)
}

// serverError sends a generic 500 without leaking internal detail to the client.
// The panic-free error paths in the auth handlers all funnel through here.
func serverError(w http.ResponseWriter) {
	http.Error(w, "internal error", http.StatusInternalServerError)
}
