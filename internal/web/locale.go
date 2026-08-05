package web

import (
	"context"
	"net/http"
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
// sits inside the session middleware and outside requireAuth, because the login
// page needs a locale too — from Accept-Language, no trainer being known yet.
func (s *Server) resolveLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), localeContextKey{}, s.localeFor(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// localeFor is the trainer's stored choice, else Accept-Language, else German. A
// trainer who never chose (empty column) or whose stored value is no longer
// supported is treated like a first visit.
func (s *Server) localeFor(r *http.Request) i18n.Locale {
	if id := s.sessions.GetInt64(r.Context(), sessionKeyTrainerID); id != 0 {
		if tr, err := store.TrainerByID(s.db, id); err == nil {
			if locale, ok := i18n.Parse(tr.Locale); ok {
				return locale
			}
		}
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
	id := s.sessions.GetInt64(r.Context(), sessionKeyTrainerID)
	if err := store.UpdateTrainerLocale(s.db, id, string(locale)); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, returnPath(r.PostFormValue(returnField)))
}

// returnPath keeps the submitted target only if it is a path within this app.
// Anything that could leave the site — an absolute URL, a protocol-relative
// "//host" or its backslash variant — or split the response — a newline — lands
// on the home page rather than in a Location header.
func returnPath(p string) string {
	switch {
	case !strings.HasPrefix(p, "/"),
		strings.HasPrefix(p, "//"),
		strings.HasPrefix(p, `/\`),
		strings.ContainsAny(p, "\r\n"):
		return homePath
	}
	return p
}
