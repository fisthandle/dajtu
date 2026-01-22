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
	cfg       *config.Config
	db        *storage.DB
	fs        *storage.Filesystem
	processor *image.Processor
}

func NewUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem, processor *image.Processor) *UploadHandler {
	return &UploadHandler{cfg: cfg, db: db, fs: fs, processor: processor}
}

type UploadResponse struct {
	Slug      string            `json:"slug"`
	URL       string            `json:"url,omitempty"`
	Sizes     map[string]string `json:"sizes,omitempty"`
	EditToken string            `json:"edit_token,omitempty"`
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
	results, err := h.processor.ProcessWithTransform(data, transformParams)
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

	editToken, err := generateEditToken()
	if err != nil {
		log.Printf("generate token error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "token generation error", http.StatusInternalServerError)
		return
	}

	img := &storage.Image{
		Slug:         slug,
		OriginalName: header.Filename,
		MimeType:     string(format),
		FileSize:     totalSize,
		Width:        originalResult.Width,
		Height:       originalResult.Height,
		CreatedAt:    now,
		AccessedAt:   now,
		EditToken:    editToken,
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
		Slug:      slug,
		URL:       sizes["original"],
		Sizes:     sizes,
		EditToken: editToken,
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

func NewImageViewHandler(db *storage.DB, baseURL string) *ImageViewHandler {
	tmpl := template.Must(template.ParseFS(templates, "templates/image.html", "templates/partials/*.html"))
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

	editToken := r.URL.Query().Get("edit")
	editMode := editToken != "" && img.EditToken != "" && editToken == img.EditToken

	data := map[string]interface{}{
		"Image":     img,
		"BaseURL":   h.baseURL,
		"CanEdit":   canEdit,
		"EditToken": editToken,
		"EditMode":  editMode,
	}

	if err := h.tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "Internal server error", 500)
	}
}

type ImageEditHandler struct {
	db        *storage.DB
	fs        *storage.Filesystem
	tmpl      *template.Template
	processor *image.Processor
	cfg       *config.Config
}

func NewImageEditHandler(db *storage.DB, fs *storage.Filesystem, tmpl *template.Template, processor *image.Processor, cfg *config.Config) *ImageEditHandler {
	return &ImageEditHandler{db: db, fs: fs, tmpl: tmpl, processor: processor, cfg: cfg}
}

func (h *ImageEditHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, slug string) {
	img, err := h.db.GetImageBySlug(slug)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}

	user := middleware.GetUser(r)
	if user == nil || img.UserID == nil || user.ID != *img.UserID {
		http.Error(w, "Forbidden", 403)
		return
	}

	if r.Method == "GET" {
		h.tmpl.ExecuteTemplate(w, "edit_image.html", map[string]interface{}{
			"Image": img,
		})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file", 400)
		return
	}
	defer file.Close()

	maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
	_, data, err := image.ValidateAndDetect(file, maxSize)
	if err != nil {
		http.Error(w, "Invalid file", 400)
		return
	}

	mode := r.FormValue("mode")

	if mode == "new" {
		newSlug := h.db.GenerateUniqueSlug("images", 5)

		transformParams := parseTransformParams(r)
		results, err := h.processor.ProcessWithTransform(data, transformParams)
		if err != nil {
			http.Error(w, "Process error", 500)
			return
		}

		for _, res := range results {
			if err := h.fs.Save(newSlug, res.Name, res.Data); err != nil {
				h.fs.Delete(newSlug)
				http.Error(w, "Save error", 500)
				return
			}
		}

		originalResult := results[0]
		newImg := &storage.Image{
			Slug:         newSlug,
			OriginalName: img.OriginalName,
			MimeType:     "image/webp",
			FileSize:     int64(len(data)),
			Width:        originalResult.Width,
			Height:       originalResult.Height,
			UserID:       &user.ID,
			CreatedAt:    time.Now().Unix(),
			AccessedAt:   time.Now().Unix(),
			Edited:       true,
			GalleryID:    img.GalleryID,
		}

		if _, err := h.db.InsertImage(newImg); err != nil {
			h.fs.Delete(newSlug)
			http.Error(w, "DB error", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"slug": newSlug})
		return
	}

	if !img.Edited {
		if err := h.fs.SaveBackup(slug); err != nil {
			log.Printf("backup failed: %v", err)
		}
	}

	transformParams := parseTransformParams(r)
	results, err := h.processor.ProcessWithTransform(data, transformParams)
	if err != nil {
		http.Error(w, "Process error", 500)
		return
	}

	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			http.Error(w, "Save error", 500)
			return
		}
	}

	// Update metadata with new dimensions and file size
	totalSize := int64(0)
	var origWidth, origHeight int
	for _, res := range results {
		totalSize += int64(len(res.Data))
		if res.Name == "original" {
			origWidth = res.Width
			origHeight = res.Height
		}
	}
	if origWidth > 0 && origHeight > 0 {
		if err := h.db.UpdateImageMetadata(slug, origWidth, origHeight, totalSize); err != nil {
			http.Error(w, "DB metadata update error", 500)
			return
		}
	}

	if err := h.db.MarkImageEdited(slug); err != nil {
		http.Error(w, "DB error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"slug": slug})
}

func (h *ImageEditHandler) RestoreOriginal(w http.ResponseWriter, r *http.Request, slug string) {
	img, err := h.db.GetImageBySlug(slug)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}

	// Must be edited to restore
	if !img.Edited {
		http.Error(w, "Image not edited", 400)
		return
	}

	// Must have backup
	if !h.fs.HasBackup(slug) {
		http.Error(w, "No backup available", 400)
		return
	}

	// Check authorization
	user := middleware.GetUser(r)
	if user == nil || img.UserID == nil || user.ID != *img.UserID {
		http.Error(w, "Forbidden", 403)
		return
	}

	// Read backup and reprocess
	backupData, err := h.fs.ReadBackup(slug)
	if err != nil {
		http.Error(w, "Restore error", 500)
		return
	}

	// Reprocess all sizes from backup
	results, err := h.processor.ProcessWithTransform(backupData, image.TransformParams{})
	if err != nil {
		http.Error(w, "Process error", 500)
		return
	}

	// Save all sizes
	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			http.Error(w, "Save error", 500)
			return
		}
	}

	// Update metadata
	totalSize := int64(0)
	var origWidth, origHeight int
	for _, res := range results {
		totalSize += int64(len(res.Data))
		if res.Name == "original" {
			origWidth = res.Width
			origHeight = res.Height
		}
	}
	if origWidth > 0 && origHeight > 0 {
		if err := h.db.UpdateImageMetadata(slug, origWidth, origHeight, totalSize); err != nil {
			http.Error(w, "DB metadata update error", 500)
			return
		}
	}

	// Clear edited flag
	if err := h.db.UnmarkImageEdited(slug); err != nil {
		http.Error(w, "DB error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *UploadHandler) DeleteImage(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		editToken = r.FormValue("edit_token")
	}

	img, err := h.db.GetImageBySlug(slug)
	if err != nil || img == nil {
		http.NotFound(w, r)
		return
	}

	if img.EditToken == "" || img.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	if err := h.fs.Delete(slug); err != nil {
		log.Printf("delete files error: %v", err)
	}

	if err := h.db.DeleteImageBySlug(slug); err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": slug})
}
