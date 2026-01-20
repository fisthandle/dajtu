package handler

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

type UserHandler struct {
	cfg      *config.Config
	db       *storage.DB
	userTmpl *template.Template
}

func NewUserHandler(cfg *config.Config, db *storage.DB) *UserHandler {
	userTmpl := template.Must(template.ParseFS(templates, "templates/user.html"))
	return &UserHandler{cfg: cfg, db: db, userTmpl: userTmpl}
}

func (h *UserHandler) View(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/u/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) > 1 && parts[1] != "" {
		http.NotFound(w, r)
		return
	}
	slug := parts[0]

	user, err := h.db.GetUserBySlug(slug)
	if err != nil || user == nil {
		http.NotFound(w, r)
		return
	}

	galleries, _ := h.db.GetUserGalleries(user.ID)

	type GalleryData struct {
		Slug      string
		Title     string
		URL       string
		CreatedAt string
	}

	var galleryList []GalleryData
	for _, g := range galleries {
		title := g.Title
		if title == "" {
			title = "Galeria " + g.Slug
		}
		galleryList = append(galleryList, GalleryData{
			Slug:      g.Slug,
			Title:     title,
			URL:       h.cfg.BaseURL + "/g/" + g.Slug,
			CreatedAt: time.Unix(g.CreatedAt, 0).Format("2006-01-02 15:04"),
		})
	}

	data := map[string]any{
		"DisplayName": user.DisplayName,
		"Galleries":   galleryList,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.userTmpl.Execute(w, data)
}
