package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/storage"
	"dajtu/internal/testutil"
)

func testSetup(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, func()) {
	t.Helper()
	dir := t.TempDir()

	cfg := &config.Config{
		Port:          "8080",
		DataDir:       dir,
		MaxFileSizeMB: 10,
		MaxDiskGB:     1.0,
		CleanupTarget: 0.5,
		BaseURL:       "http://test.local",
		PublicUpload:  true,
	}

	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	fs, err := storage.NewFilesystem(dir)
	if err != nil {
		t.Fatalf("create fs: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return cfg, db, fs, cleanup
}

func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write(content)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadHandler_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/upload", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestUploadHandler_NoFile(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	// Create valid multipart form without "file" field
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("other", "value") // Some other field, not "file"
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUploadHandler_InvalidFormat(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	// Send a text file instead of image
	content := []byte("this is not an image")
	req := createMultipartRequest(t, "file", "test.txt", content)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid image format" {
		t.Errorf("error = %q, want 'invalid image format'", resp["error"])
	}
}

func TestJsonError(t *testing.T) {
	rec := httptest.NewRecorder()
	jsonError(rec, "test error", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", rec.Header().Get("Content-Type"))
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "test error" {
		t.Errorf("error = %q, want 'test error'", resp["error"])
	}
}

func TestUploadHandler_GenerateUniqueSlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	slug := h.db.GenerateUniqueSlug("images", 5)
	if len(slug) != 5 {
		t.Errorf("slug length = %d, want 5", len(slug))
	}

	// Should not exist in db
	exists, _ := db.SlugExists("images", slug)
	if exists {
		t.Error("generated slug already exists")
	}
}

func TestUploadHandler_GenerateUniqueSlug_Collision(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	// Pre-insert many slugs to increase collision probability
	for i := 0; i < 100; i++ {
		slug := storage.GenerateSlug(5)
		img := &storage.Image{
			Slug:       slug,
			MimeType:   "image/jpeg",
			FileSize:   100,
			CreatedAt:  1,
			AccessedAt: 1,
		}
		db.InsertImage(img)
	}

	// Should still generate unique slug
	slug := h.db.GenerateUniqueSlug("images", 5)
	exists, _ := db.SlugExists("images", slug)
	if exists {
		t.Error("generated slug should be unique")
	}
}

func TestUploadHandler_SavesOriginal(t *testing.T) {
	if _, err := image.Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()
	cfg.KeepOriginalFormat = true

	h := NewUploadHandler(cfg, db, fs)

	req := createMultipartRequest(t, "file", "test.jpg", testutil.SampleJPEG())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp UploadResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, err := fs.GetOriginalPath(resp.Slug, "original"); err != nil {
		t.Fatalf("original file not found: %v", err)
	}
}
