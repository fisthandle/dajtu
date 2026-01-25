package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/handler"
	"dajtu/internal/image"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"
)

func isValidSlug(s string) bool {
	if len(s) < 2 || len(s) > 10 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func buildImageMux(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Port:               "8080",
		DataDir:            dir,
		MaxFileSizeMB:      10,
		MaxDiskGB:          1.0,
		CleanupTarget:      0.5,
		BaseURL:            "http://localhost:8080",
		KeepOriginalFormat: true,
	}
	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fs, err := storage.NewFilesystem(dir)
	if err != nil {
		t.Fatalf("NewFilesystem() error = %v", err)
	}
	processor := image.NewProcessor()

	uploadHandler := handler.NewUploadHandler(cfg, db, fs, processor)
	imageViewHandler := handler.NewImageViewHandler(db, cfg)
	imageEditHandler := handler.NewImageEditHandler(db, fs, processor, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/i/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/i/")
		parts := strings.Split(path, "/")

		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		if len(parts) > 2 {
			http.NotFound(w, r)
			return
		}

		slug := strings.TrimSuffix(parts[0], ".webp")
		if !isValidSlug(slug) {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodDelete && len(parts) == 1 {
			uploadHandler.DeleteImage(w, r, slug)
			return
		}

		if len(parts) == 2 && parts[1] == "edit" {
			imageEditHandler.ServeHTTP(w, r, slug)
			return
		}

		if len(parts) == 2 && parts[1] == "restore" {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			imageEditHandler.RestoreOriginal(w, r, slug)
			return
		}

		if len(parts) == 1 && !strings.HasSuffix(parts[0], ".webp") {
			imageViewHandler.ServeHTTP(w, r, slug)
			return
		}

		go func() {
			_ = db.TouchImageBySlug(slug)
			_ = db.IncrementDownloads(slug)
		}()

		validSizes := map[string]bool{
			"original.webp": true, "1920.webp": true, "800.webp": true,
			"200.webp": true, "thumb.webp": true,
		}

		prefix := slug[:2]
		size := "original.webp"

		if len(parts) == 2 {
			size = parts[1]
			if !strings.HasSuffix(size, ".webp") {
				size = size + ".webp"
			}
			if !validSizes[size] {
				http.NotFound(w, r)
				return
			}
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "image/webp")

		filePath := cfg.DataDir + "/images/" + prefix + "/" + slug + "/" + size
		http.ServeFile(w, r, filePath)
	})

	return cfg, db, fs, middleware.NewSessionMiddleware(db).Middleware(mux)
}

func TestImageRoute_InvalidSlugTraversal(t *testing.T) {
	_, _, _, h := buildImageMux(t)

	req := httptest.NewRequest("GET", "/i/../original", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusMovedPermanently || rec.Code == http.StatusPermanentRedirect {
		loc := rec.Header().Get("Location")
		req2 := httptest.NewRequest("GET", loc, nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotFound {
			t.Fatalf("redirect status = %d, want %d", rec2.Code, http.StatusNotFound)
		}
		return
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestImageRoute_InvalidSlugTooShort(t *testing.T) {
	_, _, _, h := buildImageMux(t)

	req := httptest.NewRequest("GET", "/i/a", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestImageRoute_ServesExistingFile(t *testing.T) {
	_, db, fs, h := buildImageMux(t)

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         "ab123",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     4,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	if err := fs.Save("ab123", "original", []byte("data")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/i/ab123/original.webp", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/webp" {
		t.Fatalf("Content-Type = %q, want %q", ct, "image/webp")
	}
}
