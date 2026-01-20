package handler

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/storage"
)

//go:embed templates/*
var templates embed.FS

type GalleryHandler struct {
	cfg           *config.Config
	db            *storage.DB
	fs            *storage.Filesystem
	galleryTmpl   *template.Template
	indexTmpl     *template.Template
}

func NewGalleryHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *GalleryHandler {
	galleryTmpl := template.Must(template.ParseFS(templates, "templates/gallery.html"))
	indexTmpl := template.Must(template.ParseFS(templates, "templates/index.html"))
	return &GalleryHandler{cfg: cfg, db: db, fs: fs, galleryTmpl: galleryTmpl, indexTmpl: indexTmpl}
}

// GET / - index page with upload form
func (h *GalleryHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.indexTmpl.Execute(w, nil)
}

type GalleryCreateResponse struct {
	Slug      string           `json:"slug"`
	URL       string           `json:"url"`
	EditToken string           `json:"edit_token"`
	Images    []UploadResponse `json:"images"`
}

// POST /gallery - create new gallery with images
func (h *GalleryHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10) // 10x for multiple files

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}

	// Create gallery (4-char slug)
	gallerySlug := h.generateUniqueSlug("galleries", 4)
	editToken := storage.GenerateSlug(32)
	now := time.Now().Unix()

	gallery := &storage.Gallery{
		Slug:        gallerySlug,
		EditToken:   editToken,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	galleryID, err := h.db.InsertGallery(gallery)
	if err != nil {
		log.Printf("db error: %v", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	var uploadedImages []UploadResponse

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue // skip invalid files
		}

		results, err := image.Process(data)
		if err != nil {
			continue
		}

		slug := h.generateUniqueSlug("images", 5)

		var totalSize int64
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &galleryID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			if res.Name == "original" {
				sizes[res.Name] = baseURL + "/i/" + slug + ".webp"
			} else {
				sizes[res.Name] = baseURL + "/i/" + slug + "/" + res.Name + ".webp"
			}
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	if len(uploadedImages) == 0 {
		jsonError(w, "no valid images uploaded", http.StatusBadRequest)
		return
	}

	resp := GalleryCreateResponse{
		Slug:      gallerySlug,
		URL:       baseURL + "/g/" + gallerySlug,
		EditToken: editToken,
		Images:    uploadedImages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /gallery/:slug/add - add images to gallery
func (h *GalleryHandler) AddImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract gallery slug from path: /gallery/XXXX/add
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "add" {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		editToken = r.FormValue("edit_token")
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	if gallery.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	// Process same as Create but with existing gallery
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10)

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	now := time.Now().Unix()
	var uploadedImages []UploadResponse

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue
		}

		results, err := image.Process(data)
		if err != nil {
			continue
		}

		slug := h.generateUniqueSlug("images", 5)

		var totalSize int64
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &gallery.ID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			if res.Name == "original" {
				sizes[res.Name] = baseURL + "/i/" + slug + ".webp"
			} else {
				sizes[res.Name] = baseURL + "/i/" + slug + "/" + res.Name + ".webp"
			}
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"added": uploadedImages,
	})
}

// DELETE /gallery/:slug/:img_slug - remove image from gallery
func (h *GalleryHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slugs from path: /gallery/XXXX/YYYYY
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]
	imageSlug := parts[1]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		r.ParseForm()
		editToken = r.FormValue("edit_token")
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	if gallery.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	// Verify image belongs to gallery
	img, err := h.db.GetImageBySlug(imageSlug)
	if err != nil || img == nil || img.GalleryID == nil || *img.GalleryID != gallery.ID {
		http.NotFound(w, r)
		return
	}

	// Delete from filesystem and database
	h.fs.Delete(imageSlug)
	h.db.DeleteImageBySlug(imageSlug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": imageSlug})
}

// GET /g/:slug - view gallery
func (h *GalleryHandler) View(w http.ResponseWriter, r *http.Request) {
	gallerySlug := strings.TrimPrefix(r.URL.Path, "/g/")
	if gallerySlug == "" {
		http.NotFound(w, r)
		return
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	images, err := h.db.GetGalleryImages(gallery.ID)
	if err != nil {
		http.Error(w, "error loading images", http.StatusInternalServerError)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	type ImageData struct {
		Slug     string
		URL      string
		ThumbURL string
		Width    int
		Height   int
	}

	var imageData []ImageData
	for _, img := range images {
		imageData = append(imageData, ImageData{
			Slug:     img.Slug,
			URL:      baseURL + "/i/" + img.Slug + ".webp",
			ThumbURL: baseURL + "/i/" + img.Slug + "/200.webp",
			Width:    img.Width,
			Height:   img.Height,
		})
	}

	data := map[string]any{
		"Title":       gallery.Title,
		"Description": gallery.Description,
		"Images":      imageData,
		"BaseURL":     baseURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.galleryTmpl.Execute(w, data)
}

func (h *GalleryHandler) generateUniqueSlug(table string, length int) string {
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = storage.GenerateSlug(length)
	}

	for _, slug := range candidates {
		exists, _ := h.db.SlugExists(table, slug)
		if !exists {
			return slug
		}
	}
	return h.generateUniqueSlug(table, length)
}
