package handler

import (
	"html/template"
	"net/http"
	"strconv"

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

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListUsers(100, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.tmpl.ExecuteTemplate(w, "users.html", users)
}

func (h *AdminHandler) Galleries(w http.ResponseWriter, r *http.Request) {
	galleries, err := h.db.ListGalleriesAdmin(100, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.tmpl.ExecuteTemplate(w, "galleries.html", galleries)
}

func (h *AdminHandler) DeleteGallery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	images, _ := h.db.GetImagesByGallery(id)
	h.db.DeleteGalleryByID(id)

	for _, img := range images {
		h.fs.Delete(img.Slug)
	}

	http.Redirect(w, r, "/admin/galleries", http.StatusSeeOther)
}

func (h *AdminHandler) Images(w http.ResponseWriter, r *http.Request) {
	images, err := h.db.ListImagesAdmin(100, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.tmpl.ExecuteTemplate(w, "images.html", images)
}

func (h *AdminHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	img, err := h.db.GetImageByID(id)
	if err == nil && img != nil {
		h.db.DeleteImageByID(id)
		h.fs.Delete(img.Slug)
	}

	http.Redirect(w, r, "/admin/images", http.StatusSeeOther)
}
