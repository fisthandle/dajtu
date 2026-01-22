package handler

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"dajtu/internal/storage"
)

type AdminHandler struct {
	db                *storage.DB
	fs                *storage.Filesystem
	dashboardTmpl     *template.Template
	usersTmpl         *template.Template
	userDetailTmpl    *template.Template
	galleriesTmpl     *template.Template
	galleryDetailTmpl *template.Template
	imagesTmpl        *template.Template
}

func NewAdminHandler(db *storage.DB, fs *storage.Filesystem) *AdminHandler {
	funcMap := template.FuncMap{
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"formatDate": func(ts int64) string {
			if ts == 0 {
				return "-"
			}
			return time.Unix(ts, 0).Format("2006-01-02 15:04")
		},
	}
	parseAdmin := func(name, file string) *template.Template {
		return template.Must(template.New(name).Funcs(funcMap).ParseFS(
			templates,
			"templates/admin/base.html",
			file,
		))
	}
	return &AdminHandler{
		db:                db,
		fs:                fs,
		dashboardTmpl:     parseAdmin("dashboard", "templates/admin/dashboard.html"),
		usersTmpl:         parseAdmin("users", "templates/admin/users.html"),
		userDetailTmpl:    parseAdmin("user_detail", "templates/admin/user_detail.html"),
		galleriesTmpl:     parseAdmin("galleries", "templates/admin/galleries.html"),
		galleryDetailTmpl: parseAdmin("gallery_detail", "templates/admin/gallery_detail.html"),
		imagesTmpl:        parseAdmin("images", "templates/admin/images.html"),
	}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.dashboardTmpl.ExecuteTemplate(w, "dashboard.html", stats)
}

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListUsers(100, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.usersTmpl.ExecuteTemplate(w, "users.html", users)
}

func (h *AdminHandler) UserDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}

	user, err := h.db.GetUserAdmin(slug)
	if err != nil || user == nil {
		http.NotFound(w, r)
		return
	}

	galleries, _ := h.db.GetUserGalleriesAdmin(user.ID)
	images, _ := h.db.GetUserImagesAdmin(user.ID, 100, 0)

	h.userDetailTmpl.ExecuteTemplate(w, "user_detail.html", map[string]any{
		"User":      user,
		"Galleries": galleries,
		"Images":    images,
	})
}

func (h *AdminHandler) Galleries(w http.ResponseWriter, r *http.Request) {
	galleries, err := h.db.ListGalleriesAdmin(100, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.galleriesTmpl.ExecuteTemplate(w, "galleries.html", galleries)
}

func (h *AdminHandler) GalleryDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}

	gallery, err := h.db.GetGalleryAdmin(slug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	images, _ := h.db.GetGalleryImagesAdmin(gallery.ID)

	h.galleryDetailTmpl.ExecuteTemplate(w, "gallery_detail.html", map[string]any{
		"Gallery": gallery,
		"Images":  images,
	})
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
	h.imagesTmpl.ExecuteTemplate(w, "images.html", images)
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
