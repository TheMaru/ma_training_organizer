package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// localeContextKey is the request-context key holding the resolved locale. It is
// an unexported type so no other package can collide with it.
type localeContextKey struct{}

const (
	languagePath = "/account/language"
	localeField  = "locale"
	returnField  = "return"
	homePath     = "/"
)

// resolveLocale puts the request's UI language in its context (ADR-0008). It
// sits inside resolveTrainer and outside requireAuth, because the login page
// needs a locale too — from Accept-Language, no trainer being known yet.
func (s *Server) resolveLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), localeContextKey{}, s.localeFor(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// localeFor is the trainer's stored choice, else Accept-Language, else German. A
// trainer who never chose (empty column) or whose stored value is no longer
// supported is treated like a first visit.
//
// It reads the trainer resolveTrainer already loaded rather than the store, so
// resolving a locale costs no query of its own. The fallback reads "no trainer in
// the context, so Accept-Language", and what it is for is the login page, which
// has to render in some language whether a trainer is known or not.
func (s *Server) localeFor(r *http.Request) i18n.Locale {
	if locale, ok := i18n.Parse(trainerOf(r.Context()).Locale); ok {
		return locale
	}
	return i18n.Match(r.Header.Get("Accept-Language"))
}

// localeOf reads the locale resolveLocale put in the context. Requests that never
// passed the middleware (static assets, the health check) get the default.
func localeOf(ctx context.Context) i18n.Locale {
	locale, ok := ctx.Value(localeContextKey{}).(i18n.Locale)
	if !ok {
		return i18n.Default
	}
	return locale
}

// translate looks a key up in the request's locale, for the messages handlers
// produce themselves rather than through a template.
func translate(r *http.Request, key string, args ...any) string {
	return i18n.T(localeOf(r.Context()), key, args...)
}

// returnTarget is where switching the language on this page leads back to. It is
// rebuilt from the normalised query rather than echoed from the request, so
// a junk or stale parameter dies here rather than travelling through the client.
// Only a GET is a place a link can return to; a page re-rendered from a POST
// offers the home page instead, its unsaved input being lost either way.
func returnTarget(r *http.Request) string {
	if r.Method != http.MethodGet {
		return homePath
	}
	return rosterQueryFrom(r).path(r.URL.Path)
}

// handleLanguage stores the submitted language on the trainer's account and
// returns them to the page they switched from. An unsupported value is not an
// error — it resolves to the default, as an unknown stored value does.
func (s *Server) handleLanguage(w http.ResponseWriter, r *http.Request) {
	locale, _ := i18n.Parse(r.PostFormValue(localeField))
	id := s.sessions.TrainerID(r.Context())
	if err := store.UpdateTrainerLocale(s.db, id, string(locale)); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, returnPath(r.PostFormValue(returnField)))
}

// returnPath keeps the submitted target only if it is a bare path within this
// app. Anything else — an absolute URL, a protocol-relative "//host" or its
// backslash variant, an opaque "mailto:" target, a control character — lands on
// the home page rather than in a Location header.
//
// It parses instead of matching prefixes, because a prefix test can only reject
// the shapes somebody thought of. A browser strips ASCII tab, LF and CR from a
// URL before parsing it (WHATWG URL standard), so "/<TAB>/host" reaches it as
// the protocol-relative "//host" — and nothing on the Go side objects, an HTAB
// being legal in an HTTP/1 field value. FuzzReturnPath pins that property.
func returnPath(p string) string {
	// Decoded first, and before parsing: url.Parse rejects a raw control byte
	// itself, so testing p alone would be a branch that never runs and would miss
	// the "%09" spelling besides.
	decoded, err := url.PathUnescape(p)
	if err != nil || strings.ContainsFunc(decoded, isControl) {
		return homePath
	}
	u, err := url.Parse(p)
	if err != nil {
		return homePath
	}
	switch {
	case u.Scheme != "", u.Host != "", u.Opaque != "",
		!strings.HasPrefix(u.Path, "/"),
		strings.HasPrefix(u.Path, "//"),
		strings.HasPrefix(u.Path, `/\`):
		return homePath
	}
	if u.RawQuery == "" {
		return u.EscapedPath()
	}
	return u.EscapedPath() + "?" + u.RawQuery
}

// isControl reports whether r is a C0 control or DEL. Stated in full rather than
// as CR and LF alone: which of them a browser or a proxy folds away is not a list
// worth betting a redirect target on.
func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}
