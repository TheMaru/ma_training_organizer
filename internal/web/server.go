// Package web wires the HTTP server: routing, templates, static assets and
// session auth (issue 04).
package web

import (
	"database/sql"
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/TheMaru/ma_training_organizer/internal/session"
)

//go:embed static/*
var staticFS embed.FS

// Server holds the shared dependencies for HTTP handlers and builds the router.
type Server struct {
	tmpl     *renderer
	db       *sql.DB
	sessions *session.Manager
}

// NewServer constructs a Server with its templates parsed. It takes the database
// (for credential lookups) and a session manager (for login state).
func NewServer(db *sql.DB, sessions *session.Manager) (*Server, error) {
	tmpl, err := newRenderer()
	if err != nil {
		return nil, err
	}
	return &Server{tmpl: tmpl, db: db, sessions: sessions}, nil
}

// Handler returns the fully-wired HTTP handler.
//
// Static assets and the health check are public and sit outside the session
// middleware. Everything else runs inside it: /login is reachable
// unauthenticated, and the remaining app routes are gated by requireAuth, which
// redirects anonymous requests to the login page.
//
// There is deliberately no client-IP middleware: chi's middleware.RealIP reads
// the address from headers the client itself sends, so r.RemoteAddr would name
// whatever the caller typed. Anything later keyed on the address wants the
// trusted-header route in issue 05, not that value back.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(staticSub)))
	r.Get("/healthz", s.handleHealthz)

	r.Group(func(r chi.Router) {
		r.Use(s.sessions.Middleware)
		r.Use(s.resolveLocale)

		r.Get("/login", s.handleLoginForm)
		r.Post("/login", s.handleLogin)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/", s.handleHome)
			r.Post("/logout", s.handleLogout)
			r.Get(passwordPath, s.handlePasswordForm)
			r.Post(passwordPath, s.handleChangePassword)
			r.Post(revokePath, s.handleRevokeSessions)
			r.Post(languagePath, s.handleLanguage)

			r.Get("/athletes", s.handleAthletesList)
			r.Get("/athletes/new", s.handleAthleteNew)
			r.Post("/athletes", s.handleAthleteCreate)
			r.Get("/athletes/{id}", s.handleAthleteDetail)
			r.Get("/athletes/{id}/edit", s.handleAthleteEdit)
			r.Post("/athletes/{id}", s.handleAthleteUpdate)
			r.Post("/athletes/{id}/delete", s.handleAthleteDelete)
			r.Post("/athletes/{id}/promotions", s.handleAthletePromote)
		})
	})

	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	// Reachable only through requireAuth, so the viewer is always authenticated.
	s.tmpl.render(w, r, http.StatusOK, "home.html", map[string]any{"Authenticated": true})
}

// redirect sends the client to the target path. For an HTMX request it uses the
// HX-Redirect header so the browser performs a full-page navigation (rather than
// swapping the redirected page into a fragment); otherwise it issues a 303 See
// Other, which turns the follow-up request into a GET.
func redirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
