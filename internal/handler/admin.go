package handler

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"
)

type AdminHandler struct {
	cfg               *config.Config
	db                *storage.DB
	fs                *storage.Filesystem
	traffic           *middleware.TrafficStats
	dashboardTmpl     *template.Template
	usersTmpl         *template.Template
	userDetailTmpl    *template.Template
	galleriesTmpl     *template.Template
	galleryDetailTmpl *template.Template
	imagesTmpl        *template.Template
	logsTmpl          *template.Template
}

func NewAdminHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem, traffic *middleware.TrafficStats) *AdminHandler {
	funcMap := template.FuncMap{
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"formatBytes": func(b int64) string {
			const (
				kb = 1024
				mb = 1024 * 1024
				gb = 1024 * 1024 * 1024
			)
			switch {
			case b >= gb:
				return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
			case b >= mb:
				return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
			case b >= kb:
				return fmt.Sprintf("%.2f KB", float64(b)/float64(kb))
			default:
				return fmt.Sprintf("%d B", b)
			}
		},
		"formatCount": func(n int64) string {
			switch {
			case n >= 1_000_000_000:
				return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
			case n >= 1_000_000:
				return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
			case n >= 1_000:
				return fmt.Sprintf("%.1fK", float64(n)/1_000)
			default:
				return fmt.Sprintf("%d", n)
			}
		},
		"formatRate": func(bps float64) string {
			const (
				kb = 1024.0
				mb = 1024.0 * 1024.0
				gb = 1024.0 * 1024.0 * 1024.0
			)
			switch {
			case bps >= gb:
				return fmt.Sprintf("%.2f GB/s", bps/gb)
			case bps >= mb:
				return fmt.Sprintf("%.2f MB/s", bps/mb)
			case bps >= kb:
				return fmt.Sprintf("%.2f KB/s", bps/kb)
			default:
				return fmt.Sprintf("%.2f B/s", bps)
			}
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
		cfg:               cfg,
		db:                db,
		fs:                fs,
		traffic:           traffic,
		dashboardTmpl:     parseAdmin("dashboard", "templates/admin/dashboard.html"),
		usersTmpl:         parseAdmin("users", "templates/admin/users.html"),
		userDetailTmpl:    parseAdmin("user_detail", "templates/admin/user_detail.html"),
		galleriesTmpl:     parseAdmin("galleries", "templates/admin/galleries.html"),
		galleryDetailTmpl: parseAdmin("gallery_detail", "templates/admin/gallery_detail.html"),
		imagesTmpl:        parseAdmin("images", "templates/admin/images.html"),
		logsTmpl:          parseAdmin("logs", "templates/admin/logs.html"),
	}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var traffic middleware.TrafficSnapshot
	if h.traffic != nil {
		traffic = h.traffic.Snapshot(time.Now())
	}
	data := map[string]any{
		"Stats":   stats,
		"Traffic": traffic,
	}
	h.dashboardTmpl.ExecuteTemplate(w, "dashboard.html", data)
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

func (h *AdminHandler) Logs(w http.ResponseWriter, r *http.Request) {
	lines := 300
	if raw := r.URL.Query().Get("lines"); raw != "" {
		if raw == "all" {
			lines = 0
		} else {
			lines = parseQueryInt(r, "lines", 300)
			if lines < 1 {
				lines = 300
			}
			if lines > 5000 {
				lines = 5000
			}
		}
	}

	files, err := listLogFiles(h.cfg.LogDir)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	selected := r.URL.Query().Get("file")
	if selected == "" && len(files) > 0 {
		selected = files[0].Name
	}
	if selected != "" && !fileAllowed(selected, files) {
		http.Error(w, "invalid log file", http.StatusBadRequest)
		return
	}

	var content string
	var readErr string
	if selected != "" {
		path := filepath.Join(h.cfg.LogDir, selected)
		if lines == 0 {
			data, err := os.ReadFile(path)
			if err != nil {
				readErr = err.Error()
			} else {
				content = string(data)
			}
		} else {
			linesData, err := tailLines(path, lines)
			if err != nil {
				readErr = err.Error()
			} else {
				content = strings.Join(linesData, "\n")
			}
		}
	}

	data := map[string]any{
		"Files":    files,
		"Selected": selected,
		"Lines":    lines,
		"Content":  content,
		"Error":    readErr,
	}
	h.logsTmpl.ExecuteTemplate(w, "logs.html", data)
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

type logFileInfo struct {
	Name    string
	Size    int64
	Lines   int64
	ModUnix int64
}

func listLogFiles(dir string) ([]logFileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []logFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".log") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			lineCount, err := countLines(filepath.Join(dir, name))
			if err != nil {
				lineCount = 0
			}
			files = append(files, logFileInfo{
				Name:    name,
				Size:    info.Size(),
				Lines:   lineCount,
				ModUnix: info.ModTime().Unix(),
			})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name > files[j].Name
	})
	return files, nil
}

func fileAllowed(name string, allowed []logFileInfo) bool {
	if filepath.Base(name) != name {
		return false
	}
	for _, f := range allowed {
		if f.Name == name {
			return true
		}
	}
	return false
}

func tailLines(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return []string{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return []string{}, nil
	}

	const chunkSize int64 = 32 * 1024
	var (
		offset int64 = size
		buf          = make([]byte, 0)
		lines  []string
	)

	for offset > 0 && len(lines) <= maxLines {
		readSize := chunkSize
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		n, err := file.ReadAt(chunk, offset)
		if err != nil && err != io.EOF {
			return nil, err
		}
		chunk = chunk[:n]
		buf = append(chunk, buf...)
		lines = strings.Split(string(buf), "\n")
		if len(lines) > maxLines+1 {
			lines = lines[len(lines)-(maxLines+1):]
			buf = []byte(strings.Join(lines, "\n"))
		}
	}

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

func countLines(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	const bufSize = 32 * 1024
	buf := make([]byte, bufSize)
	var count int64
	var lastByte byte
	var sawData bool

	for {
		n, err := file.Read(buf)
		if n > 0 {
			sawData = true
			for _, b := range buf[:n] {
				if b == '\n' {
					count++
				}
				lastByte = b
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}

	if sawData && lastByte != '\n' {
		count++
	}

	return count, nil
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
