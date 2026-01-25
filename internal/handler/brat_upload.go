package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/auth"
	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/logging"
	"dajtu/internal/storage"
)

type BratUploadHandler struct {
	cfg       *config.Config
	db        *storage.DB
	fs        *storage.Filesystem
	decoder   *auth.BratDecoder
	processor *image.Processor
}

func NewBratUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem, decoder *auth.BratDecoder, processor *image.Processor) *BratUploadHandler {
	return &BratUploadHandler{cfg: cfg, db: db, fs: fs, decoder: decoder, processor: processor}
}

type BratUploadResponse struct {
	URL      string `json:"url"`
	ViewURL  string `json:"view_url"`
	ThumbURL string `json:"thumbUrl"`
	Filename string `json:"filename"`
	Slug     string `json:"slug"`
}

func (h *BratUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if h.cfg.IsOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/brtup/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		jsonError(w, "invalid path format", http.StatusBadRequest)
		return
	}

	token := parts[0]
	entryID := parts[1]
	titleBase64 := parts[2]

	if h.decoder == nil {
		jsonError(w, "SSO not configured", http.StatusServiceUnavailable)
		return
	}

	user, err := h.decoder.DecodeWithMaxAge(token, 86400)
	if err != nil {
		logging.Get("brat").Printf("token decode error: %v", err)
		jsonError(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	var title string
	if titleBase64 == "nope" {
		title = "Nowe wątki"
	} else {
		titleBytes, err := base64.URLEncoding.DecodeString(titleBase64)
		if err != nil {
			jsonError(w, "invalid title encoding", http.StatusBadRequest)
			return
		}
		title = string(titleBytes)
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024)

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024); err != nil {
		jsonError(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		jsonError(w, "no file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

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

	dbUser, err := h.db.GetOrCreateBratUser(user.Pseudonim)
	if err != nil {
		logging.Get("brat").Printf("get/create user error: %v", err)
		jsonError(w, "user creation failed", http.StatusInternalServerError)
		return
	}

	gallery, err := h.db.GetOrCreateBratGallery(dbUser.ID, dbUser.Slug, entryID, title)
	if err != nil {
		logging.Get("brat").Printf("get/create gallery error: %v", err)
		jsonError(w, "gallery creation failed", http.StatusInternalServerError)
		return
	}

	slug := h.db.GenerateUniqueSlug("images", 5)

	var originalSize int64
	if h.cfg.KeepOriginalFormat {
		size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
		if err != nil {
			logging.Get("brat").Printf("warning: failed to save original: %v", err)
		} else {
			originalSize = size
		}
	}

	results, err := h.processor.ProcessWithTransform(data, image.TransformParams{})
	if err != nil {
		logging.Get("brat").Printf("process error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "image processing failed", http.StatusInternalServerError)
		return
	}

	totalSize := originalSize
	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			logging.Get("brat").Printf("save error: %v", err)
			h.fs.Delete(slug)
			jsonError(w, "save failed", http.StatusInternalServerError)
			return
		}
		totalSize += int64(len(res.Data))
	}

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         slug,
		OriginalName: header.Filename,
		MimeType:     string(format),
		FileSize:     totalSize,
		Width:        results[0].Width,
		Height:       results[0].Height,
		UserID:       &dbUser.ID,
		GalleryID:    &gallery.ID,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	if _, err := h.db.InsertImage(img); err != nil {
		logging.Get("brat").Printf("insert image error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := getBaseURL(h.cfg, r)

	resp := BratUploadResponse{
		URL:      buildImageURL(baseURL, slug, "1200"),
		ViewURL:  baseURL + "/i/" + slug,
		ThumbURL: buildImageURL(baseURL, slug, "thumb"),
		Filename: header.Filename,
		Slug:     slug,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
