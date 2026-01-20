package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	// Process image (re-encode + resize)
	results, err := image.Process(data)
	if err != nil {
		log.Printf("process error: %v", err)
		jsonError(w, "image processing failed", http.StatusInternalServerError)
		return
	}

	// Generate unique slug (5 chars for images)
	slug := h.generateUniqueSlug("images", 5)

	// Save all sizes
	var totalSize int64
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
	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", r.Host)
	}

	sizes := make(map[string]string)
	for _, res := range results {
		if res.Name == "original" {
			sizes[res.Name] = fmt.Sprintf("%s/i/%s.webp", baseURL, slug)
		} else {
			sizes[res.Name] = fmt.Sprintf("%s/i/%s/%s.webp", baseURL, slug, res.Name)
		}
	}

	resp := UploadResponse{
		Slug:  slug,
		URL:   sizes["original"],
		Sizes: sizes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) generateUniqueSlug(table string, length int) string {
	// Generate 20 candidates at once to minimize DB queries
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
	// Fallback: try again (extremely unlikely to reach here)
	return h.generateUniqueSlug(table, length)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
