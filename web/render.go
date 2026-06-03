package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var StaticFS embed.FS

// Renderer parses page templates, each composed with the shared layout.
type Renderer struct {
	pages map[string]*template.Template
}

var funcs = template.FuncMap{
	"title": strings.Title,
}

// NewRenderer parses layout.html + every other template into per-page sets.
func NewRenderer() (*Renderer, error) {
	entries, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	r := &Renderer{pages: make(map[string]*template.Template)}
	for _, e := range entries {
		name := strings.TrimSuffix(strings.TrimPrefix(e, "templates/"), ".html")
		if name == "layout" {
			continue
		}
		tmpl, err := template.New("layout.html").Funcs(funcs).
			ParseFS(templateFS, "templates/layout.html", e)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", e, err)
		}
		r.pages[name] = tmpl
	}
	return r, nil
}

// Render writes a full page (layout + named page) with the given data.
func (r *Renderer) Render(w http.ResponseWriter, status int, page string, data any) {
	tmpl, ok := r.pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// RenderPartial executes a single named template (for HTMX fragment responses).
func (r *Renderer) RenderPartial(w http.ResponseWriter, page, fragment string, data any) {
	tmpl, ok := r.pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, fragment, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
