package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/auth"
	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/storage"
)

type BratUploadHandler struct {
	cfg     *config.Config
	db      *storage.DB
	fs      *storage.Filesystem
	decoder *auth.BratDecoder
}

func NewBratUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem, decoder *auth.BratDecoder) *BratUploadHandler {
	return &BratUploadHandler{cfg: cfg, db: db, fs: fs, decoder: decoder}
}

type BratUploadResponse struct {
	URL      string `json:"url"`
	ThumbURL string `json:"thumbUrl"`
	Filename string `json:"filename"`
	Slug     string `json:"slug"`
}

func (h *BratUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if strings.Contains(origin, "braterstwo.eu") || strings.Contains(origin, "localhost") {
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
		log.Printf("token decode error: %v", err)
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
		log.Printf("get/create user error: %v", err)
		jsonError(w, "user creation failed", http.StatusInternalServerError)
		return
	}

	externalID := fmt.Sprintf("brat-%s", entryID)
	gallery, err := h.db.GetGalleryByExternalID(externalID)
	if err != nil {
		log.Printf("get gallery error: %v", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	if gallery == nil {
		gallerySlug := generateUniqueSlug(h.db, "galleries", 4)
		editToken := generateEditToken()
		now := time.Now().Unix()

		gallery = &storage.Gallery{
			Slug:       gallerySlug,
			EditToken:  editToken,
			Title:      title,
			UserID:     &dbUser.ID,
			ExternalID: &externalID,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		galleryID, err := h.db.InsertGallery(gallery)
		if err != nil {
			log.Printf("insert gallery error: %v", err)
			jsonError(w, "gallery creation failed", http.StatusInternalServerError)
			return
		}
		gallery.ID = galleryID
	}

	slug := generateUniqueSlug(h.db, "images", 5)

	var originalSize int64
	if h.cfg.KeepOriginalFormat {
		size, err := h.fs.SaveOriginal(slug, "original", data, string(format))
		if err != nil {
			log.Printf("warning: failed to save original: %v", err)
		} else {
			originalSize = size
		}
	}

	results, err := image.Process(data)
	if err != nil {
		log.Printf("process error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "image processing failed", http.StatusInternalServerError)
		return
	}

	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			log.Printf("save error: %v", err)
			h.fs.Delete(slug)
			jsonError(w, "save failed", http.StatusInternalServerError)
			return
		}
	}

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         slug,
		OriginalName: header.Filename,
		MimeType:     string(format),
		FileSize:     originalSize,
		Width:        results[0].Width,
		Height:       results[0].Height,
		UserID:       &dbUser.ID,
		GalleryID:    &gallery.ID,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	if _, err := h.db.InsertImage(img); err != nil {
		log.Printf("insert image error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", r.Host)
	}

	resp := BratUploadResponse{
		URL:      fmt.Sprintf("%s/i/%s.webp", baseURL, slug),
		ThumbURL: fmt.Sprintf("%s/i/%s/thumb.webp", baseURL, slug),
		Filename: header.Filename,
		Slug:     slug,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func generateUniqueSlug(db *storage.DB, table string, length int) string {
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = storage.GenerateSlug(length)
	}

	for _, slug := range candidates {
		exists, _ := db.SlugExists(table, slug)
		if !exists {
			return slug
		}
	}
	return generateUniqueSlug(db, table, length)
}

func generateEditToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
