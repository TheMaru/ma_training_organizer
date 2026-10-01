package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/rankview"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

// templateFuncs are the helpers every page may call, bound to one locale.
//
// "t" looks a translation up in that locale. A template func cannot see the
// request, so the locale is baked into the parsed template set — one per locale
// — rather than passed at every call site (ADR-0008). The rank funcs close over
// the same locale, which is why the composition needs nothing threaded through
// handlers or store types.
func templateFuncs(locale i18n.Locale) template.FuncMap {
	return template.FuncMap{
		"rosterRank": func(rank store.Rank) template.HTML {
			return rankview.RosterRank(locale, rank)
		},
		"rankWithBelt": func(rank store.Rank) template.HTML {
			return rankview.RankWithBelt(locale, rank)
		},
		"rankName": func(rank store.Rank) string {
			return rankview.RankName(locale, rank)
		},
		"systemName": func(system store.System) string {
			return rankview.SystemName(locale, system)
		},
		"t": func(key string, args ...any) string {
			return i18n.T(locale, key, args...)
		},
	}
}

// renderer holds one fully-parsed template set per locale and page. Every page is
// parsed together with base.html, so a page only needs to (re)define the "title"
// and "content" blocks that base.html invokes.
type renderer struct {
	pages map[i18n.Locale]map[string]*template.Template
}

func newRenderer() (*renderer, error) {
	pageFiles, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	r := &renderer{pages: make(map[i18n.Locale]map[string]*template.Template, len(i18n.Supported()))}
	for _, locale := range i18n.Supported() {
		set := make(map[string]*template.Template)
		for _, file := range pageFiles {
			name := path.Base(file)
			if name == "base.html" {
				continue
			}
			t, err := template.New(name).Funcs(templateFuncs(locale)).ParseFS(templateFS, "templates/base.html", file)
			if err != nil {
				return nil, fmt.Errorf("parse template %s (%s): %w", name, locale, err)
			}
			set[name] = t
		}
		r.pages[locale] = set
	}
	return r, nil
}

// render executes the named page (e.g. "home.html") through the base layout, in
// the locale the request resolved to. It renders into a buffer first so a
// template error yields a clean 500 rather than a half-written response.
//
// Two values every page needs are filled in here rather than by each handler:
// "Lang" for <html lang>, and "Return" — where the language switcher sends the
// trainer back to (see returnTarget). A handler that knows its page's URL better
// than the request does sets "Return" itself, and keeps it.
func (r *renderer) render(w http.ResponseWriter, req *http.Request, status int, page string, data map[string]any) {
	locale := localeOf(req.Context())
	t, ok := r.pages[locale][page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	data["Lang"] = string(locale)
	if _, ok := data["Return"]; !ok {
		data["Return"] = returnTarget(req)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
