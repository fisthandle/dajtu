package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	page := parseQueryInt(r, "page", 1)
	limit := parseQueryInt(r, "limit", 300)
	if limit < 1 {
		limit = 300
	}
	if limit > 500 {
		limit = 500
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "created"
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" {
		dir = "desc"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	total, err := h.db.CountUsersFiltered(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * limit
	users, err := h.db.ListUsersSortedFiltered(limit, offset, sortBy, dir, query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defaultDir := map[string]string{
		"created": "desc",
		"name":    "asc",
		"slug":    "asc",
	}
	sortLink := func(field string) string {
		nextDir := defaultDir[field]
		if nextDir == "" {
			nextDir = "desc"
		}
		if field == sortBy {
			if dir == "asc" {
				nextDir = "desc"
			} else {
				nextDir = "asc"
			}
		}
		return fmt.Sprintf("/admin/users?page=1&limit=%d&sort=%s&dir=%s&q=%s", limit, field, nextDir, url.QueryEscape(query))
	}

	pages := adminPages(page, totalPages)
	data := map[string]any{
		"Users":      users,
		"Total":      total,
		"Page":       page,
		"PerPage":    limit,
		"TotalPages": totalPages,
		"Pages":      pages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   maxInt(page-1, 1),
		"NextPage":   minInt(page+1, totalPages),
		"Sort":       sortBy,
		"Dir":        dir,
		"Query":      query,
		"SortLinks": map[string]string{
			"created": sortLink("created"),
			"name":    sortLink("name"),
			"slug":    sortLink("slug"),
		},
	}
	h.usersTmpl.ExecuteTemplate(w, "users.html", data)
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
	page := parseQueryInt(r, "page", 1)
	limit := parseQueryInt(r, "limit", 300)
	if limit < 1 {
		limit = 300
	}
	if limit > 500 {
		limit = 500
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "created"
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" {
		dir = "desc"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	total, err := h.db.CountGalleriesAdminFiltered(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * limit
	galleries, err := h.db.ListGalleriesAdminSortedFiltered(limit, offset, sortBy, dir, query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defaultDir := map[string]string{
		"created": "desc",
		"title":   "asc",
		"images":  "desc",
	}
	sortLink := func(field string) string {
		nextDir := defaultDir[field]
		if nextDir == "" {
			nextDir = "desc"
		}
		if field == sortBy {
			if dir == "asc" {
				nextDir = "desc"
			} else {
				nextDir = "asc"
			}
		}
		return fmt.Sprintf("/admin/galleries?page=1&limit=%d&sort=%s&dir=%s&q=%s", limit, field, nextDir, url.QueryEscape(query))
	}

	pages := adminPages(page, totalPages)
	data := map[string]any{
		"Galleries":  galleries,
		"Total":      total,
		"Page":       page,
		"PerPage":    limit,
		"TotalPages": totalPages,
		"Pages":      pages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   maxInt(page-1, 1),
		"NextPage":   minInt(page+1, totalPages),
		"Sort":       sortBy,
		"Dir":        dir,
		"Query":      query,
		"SortLinks": map[string]string{
			"created": sortLink("created"),
			"title":   sortLink("title"),
			"images":  sortLink("images"),
		},
	}
	h.galleriesTmpl.ExecuteTemplate(w, "galleries.html", data)
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
	page := parseQueryInt(r, "page", 1)
	limit := parseQueryInt(r, "limit", 300)
	if limit < 1 {
		limit = 300
	}
	if limit > 500 {
		limit = 500
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "created"
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" {
		dir = "desc"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	total, err := h.db.CountImagesAdminFiltered(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * limit
	images, err := h.db.ListImagesAdminSortedFiltered(limit, offset, sortBy, dir, query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defaultDir := map[string]string{
		"created":   "desc",
		"downloads": "desc",
		"accessed":  "desc",
		"size":      "desc",
	}
	sortLink := func(field string) string {
		nextDir := defaultDir[field]
		if nextDir == "" {
			nextDir = "desc"
		}
		if field == sortBy {
			if dir == "asc" {
				nextDir = "desc"
			} else {
				nextDir = "asc"
			}
		}
		return fmt.Sprintf("/admin/images?page=1&limit=%d&sort=%s&dir=%s&q=%s", limit, field, nextDir, url.QueryEscape(query))
	}

	pages := adminPages(page, totalPages)
	data := map[string]any{
		"Images":     images,
		"Total":      total,
		"Page":       page,
		"PerPage":    limit,
		"TotalPages": totalPages,
		"Pages":      pages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   maxInt(page-1, 1),
		"NextPage":   minInt(page+1, totalPages),
		"Sort":       sortBy,
		"Dir":        dir,
		"Query":      query,
		"SortLinks": map[string]string{
			"downloads": sortLink("downloads"),
			"accessed":  sortLink("accessed"),
			"size":      sortLink("size"),
			"created":   sortLink("created"),
		},
	}
	h.imagesTmpl.ExecuteTemplate(w, "images.html", data)
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

func parseQueryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return val
}

func adminPages(current, total int) []int {
	if total <= 1 {
		return []int{1}
	}
	pages := map[int]bool{}
	add := func(p int) {
		if p >= 1 && p <= total {
			pages[p] = true
		}
	}
	add(1)
	add(total)
	for i := current - 2; i <= current+2; i++ {
		add(i)
	}
	out := make([]int, 0, len(pages))
	for p := range pages {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
