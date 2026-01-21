package handler

import (
	"html/template"
	"net/http"

	"dajtu/internal/storage"
)

type AdminHandler struct {
	db   *storage.DB
	fs   *storage.Filesystem
	tmpl *template.Template
}

func NewAdminHandler(db *storage.DB, fs *storage.Filesystem) *AdminHandler {
	funcMap := template.FuncMap{
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
	}
	return &AdminHandler{
		db:   db,
		fs:   fs,
		tmpl: template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/admin/*.html")),
	}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.tmpl.ExecuteTemplate(w, "dashboard.html", stats)
}
