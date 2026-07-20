package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
)

//go:embed templates/*.html
var templateFS embed.FS

// renderer holds one fully-parsed template set per page. Every page is parsed
// together with base.html, so a page only needs to (re)define the "title" and
// "content" blocks that base.html invokes.
type renderer struct {
	pages map[string]*template.Template
}

func newRenderer() (*renderer, error) {
	pageFiles, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	r := &renderer{pages: make(map[string]*template.Template)}
	for _, file := range pageFiles {
		name := path.Base(file)
		if name == "base.html" {
			continue
		}
		t, err := template.New(name).ParseFS(templateFS, "templates/base.html", file)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		r.pages[name] = t
	}
	return r, nil
}

// render executes the named page (e.g. "home.html") through the base layout.
// It renders into a buffer first so a template error yields a clean 500 rather
// than a half-written response.
func (r *renderer) render(w http.ResponseWriter, status int, page string, data any) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
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
