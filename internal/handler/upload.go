package handler

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"
)

type UploadHandler struct {
	cfg *config.Config
	db  *storage.DB
	fs  *storage.Filesystem
}

func NewUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *UploadHandler {
	return &UploadHandler{cfg: cfg, db: db, fs: fs}
}

type UploadResponse struct {
	Slug  string            `json:"slug"`
	URL   string            `json:"url"`
	Sizes map[string]string `json:"sizes"`
}

var extToMime = map[string]string{
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
}

func parseTransformParams(r *http.Request) image.TransformParams {
	params := image.TransformParams{}

	if rot := r.FormValue("rotation"); rot != "" {
		if value, err := strconv.Atoi(rot); err == nil {
			params.Rotation = value
		}
	}

	params.FlipH = r.FormValue("flipH") == "true"
	params.FlipV = r.FormValue("flipV") == "true"

	if v := r.FormValue("cropX"); v != "" {
		if value, err := strconv.Atoi(v); err == nil {
			params.CropX = value
		}
	}
	if v := r.FormValue("cropY"); v != "" {
		if value, err := strconv.Atoi(v); err == nil {
			params.CropY = value
		}
	}
	if v := r.FormValue("cropW"); v != "" {
		if value, err := strconv.Atoi(v); err == nil {
			params.CropW = value
		}
	}
	if v := r.FormValue("cropH"); v != "" {
		if value, err := strconv.Atoi(v); err == nil {
			params.CropH = value
		}
	}

	return params
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.cfg.PublicUpload {
		jsonError(w, "public upload disabled", http.StatusForbidden)
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024)

	// Parse multipart form
	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024); err != nil {
		jsonError(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "no file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate magic bytes and read content
	maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
	format, data, err := image.ValidateAndDetect(file, maxSize)
	if err != nil {
		if err == image.ErrInvalidFormat {
			jsonError(w, "invalid image format", http.StatusBadRequest)
			return
		}
		if err == image.ErrFileTooLarge {
			jsonError(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		jsonError(w, "validation error", http.StatusBadRequest)
		return
	}

	// Generate unique slug (5 chars for images)
	slug := h.db.GenerateUniqueSlug("images", 5)

	var originalSize int64
	if h.cfg.KeepOriginalFormat {
		size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
		if err != nil {
			log.Printf("warning: failed to save original: %v", err)
		} else {
			originalSize = size
		}
	}

	// Process image (re-encode + resize)
	transformParams := parseTransformParams(r)
	var results []image.ProcessResult
	if transformParams.HasTransforms() {
		results, err = image.ProcessWithTransform(data, transformParams)
	} else {
		results, err = image.Process(data)
	}
	if err != nil {
		log.Printf("process error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "image processing failed", http.StatusInternalServerError)
		return
	}

	// Save all sizes
	totalSize := originalSize
	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			log.Printf("save error: %v", err)
			h.fs.Delete(slug) // cleanup on failure
			jsonError(w, "storage error", http.StatusInternalServerError)
			return
		}
		totalSize += int64(len(res.Data))
	}

	// Store metadata
	now := time.Now().Unix()
	originalResult := results[0]
	img := &storage.Image{
		Slug:         slug,
		OriginalName: header.Filename,
		MimeType:     string(format),
		FileSize:     totalSize,
		Width:        originalResult.Width,
		Height:       originalResult.Height,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	if _, err := h.db.InsertImage(img); err != nil {
		log.Printf("db error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Build response
	baseURL := getBaseURL(h.cfg, r)

	sizes := make(map[string]string)
	for _, res := range results {
		sizes[res.Name] = buildImageURL(baseURL, slug, res.Name)
	}

	resp := UploadResponse{
		Slug:  slug,
		URL:   sizes["original"],
		Sizes: sizes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) ServeOriginal(w http.ResponseWriter, r *http.Request, slug string) {
	path, err := h.fs.GetOriginalPath(slug, "original")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ext := filepath.Ext(path)
	contentType := extToMime[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

type ImageViewHandler struct {
	db      *storage.DB
	tmpl    *template.Template
	baseURL string
}

func NewImageViewHandler(db *storage.DB, tmpl *template.Template, baseURL string) *ImageViewHandler {
	return &ImageViewHandler{db: db, tmpl: tmpl, baseURL: baseURL}
}

func (h *ImageViewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, slug string) {
	img, err := h.db.GetImageBySlug(slug)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}

	canEdit := false
	if user := middleware.GetUser(r); user != nil && img.UserID != nil && user.ID == *img.UserID {
		canEdit = true
	}

	data := map[string]interface{}{
		"Image":   img,
		"BaseURL": h.baseURL,
		"CanEdit": canEdit,
	}

	h.tmpl.ExecuteTemplate(w, "image.html", data)
}
