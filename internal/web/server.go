// Package web wires the HTTP server: routing, templates, static assets and
// (from issue 04 on) session auth.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static/*
var staticFS embed.FS

// Server holds the shared dependencies for HTTP handlers and builds the router.
type Server struct {
	tmpl *renderer
}

// NewServer constructs a Server with its templates parsed.
func NewServer() (*Server, error) {
	tmpl, err := newRenderer()
	if err != nil {
		return nil, err
	}
	return &Server{tmpl: tmpl}, nil
}

// Handler returns the fully-wired HTTP handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	r.Get("/healthz", s.handleHealthz)
	r.Get("/", s.handleHome)

	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleHome(w http.ResponseWriter, _ *http.Request) {
	s.tmpl.render(w, http.StatusOK, "home.html", map[string]any{"Authenticated": false})
}
