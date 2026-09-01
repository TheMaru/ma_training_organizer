package web

import (
	"context"
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

// trainerContextKey is the request-context key holding the request's trainer. It
// is an unexported type so no other package can collide with it.
type trainerContextKey struct{}

// resolveTrainer loads the session's trainer once and puts them in the request
// context, for everything downstream that has a question about them: their
// locale, whether they may be here at all, their password hash.
//
// The id in the session is not taken as proof that the account may still be used,
// so the state is read on every request. Without that, a session outlives the
// account it belongs to — deleting or deactivating a trainer would leave their
// browser with full access until the session happened to expire, and revocation
// is a separate act that cannot be relied on to have happened (ADR-0010).
//
// A trainer who may not use the app leaves the context empty rather than being
// refused here: this middleware runs outside requireAuth, because the login page
// is downstream of it too. Refusing is requireAuth's act.
func (s *Server) resolveTrainer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := s.sessions.TrainerID(r.Context())
		if id == 0 {
			next.ServeHTTP(w, r)
			return
		}
		tr, mayUse, err := auth.TrainerMayUseTheApp(s.db, id)
		if err != nil {
			serverError(w)
			return
		}
		if mayUse {
			r = r.WithContext(context.WithValue(r.Context(), trainerContextKey{}, tr))
		}
		next.ServeHTTP(w, r)
	})
}

// trainerOf reads the trainer resolveTrainer put in the context. The zero Trainer
// means nobody: no session, a session naming an account that is gone or
// deactivated, or a request that never passed the middleware at all (static
// assets, the health check). Nothing downstream needs those apart.
func trainerOf(ctx context.Context) store.Trainer {
	tr, _ := ctx.Value(trainerContextKey{}).(store.Trainer)
	return tr
}

// requireAuth gates a route: requests without a usable trainer are redirected to
// the login page instead of reaching the handler.
//
// It tells its two refusals apart by asking the session, which costs no query: no
// session at all is a plain redirect, while a session naming nobody usable is
// worth destroying first.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.sessions.TrainerID(r.Context()) == 0 {
			redirect(w, r, "/login")
			return
		}
		if trainerOf(r.Context()).ID != 0 {
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

// handleLoginForm renders the login page. An already-authenticated trainer is
// bounced to the home page rather than shown the form again.
func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if s.sessions.TrainerID(r.Context()) != 0 {
		redirect(w, r, "/")
		return
	}
	s.renderLogin(w, r, http.StatusOK, "", "")
}

// handleLogin submits the credentials to auth and, on success, starts a session.
// Every refusal renders one page with one message, so the handler gives away
// nothing auth.ErrBadCredentials withholds.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	tr, err := auth.Authenticate(s.db, username, password)
	if err != nil {
		if errors.Is(err, auth.ErrBadCredentials) {
			s.renderLogin(w, r, http.StatusUnauthorized, username, translate(r, "login.badCredentials"))
			return
		}
		serverError(w)
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

// handleChangePassword applies a new password to the trainer's own account. The
// two form fields have to agree, which is this page's question and is settled
// here; whether the credentials permit the change is auth's, and is settled
// there.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	current := r.PostFormValue("current")
	next := r.PostFormValue("new")
	confirm := r.PostFormValue("confirm")

	if next != confirm {
		s.renderPassword(w, r, http.StatusBadRequest, translate(r, "password.mismatch"))
		return
	}

	switch err := auth.ChangePassword(s.db, trainerOf(r.Context()), current, next); {
	case errors.Is(err, auth.ErrCurrentPasswordWrong):
		s.renderPassword(w, r, http.StatusUnauthorized, translate(r, "password.currentWrong"))
		return
	case errors.Is(err, auth.ErrPasswordTooShort):
		s.renderPassword(w, r, http.StatusBadRequest, translate(r, "password.tooShort"))
		return
	case err != nil:
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
	if err := s.sessions.RevokeOthers(r.Context()); err != nil {
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
