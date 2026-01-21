package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
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

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	results, err := image.Process(data)
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
