package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dajtu/internal/storage"
)

func TestGalleryHandler_Index(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q, want text/html", rec.Header().Get("Content-Type"))
	}
}

func TestGalleryHandler_Index_NotRoot(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/other", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_Create_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery", nil)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_Create_NoFiles(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "no files provided" {
		t.Errorf("error = %q, want 'no files provided'", resp["error"])
	}
}

func TestGalleryHandler_View_NotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/g/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_View_EmptySlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/g/", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_View_Success(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	// Create a gallery
	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:        "test",
		EditToken:   "token123",
		Title:       "Test Gallery",
		Description: "Description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("GET", "/g/test", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Test Gallery") {
		t.Error("response should contain gallery title")
	}
}

func TestGalleryHandler_View_EditMode(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:        "edit",
		EditToken:   "secret123",
		Title:       "Editable",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("GET", "/g/edit?edit=secret123", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGalleryHandler_AddImages_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery/test/add", nil)
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_AddImages_InvalidPath(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	tests := []string{
		"/gallery/test",       // No /add
		"/gallery/test/other", // Wrong suffix
		"/gallery/",           // Empty slug
	}

	for _, path := range tests {
		req := httptest.NewRequest("POST", path, nil)
		rec := httptest.NewRecorder()

		h.AddImages(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestGalleryHandler_AddImages_GalleryNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("POST", "/gallery/nonexistent/add", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_AddImages_InvalidToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "add1",
		EditToken: "correct",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("POST", "/gallery/add1/add", nil)
	req.Header.Set("X-Edit-Token", "wrong")
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid edit token" {
		t.Errorf("error = %q, want 'invalid edit token'", resp["error"])
	}
}

func TestGalleryHandler_DeleteImage_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery/test/img1", nil)
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_DeleteImage_InvalidPath(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("DELETE", "/gallery/test", nil)
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_GalleryNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("DELETE", "/gallery/nonexistent/img1", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_InvalidToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "del1",
		EditToken: "correct",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("DELETE", "/gallery/del1/img1", nil)
	req.Header.Set("X-Edit-Token", "wrong")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGalleryHandler_DeleteImage_ImageNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "del2",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("DELETE", "/gallery/del2/nonexistent", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_Success(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()

	g := &storage.Gallery{
		Slug:      "del4",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	img := &storage.Image{
		Slug:       "todel",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  now,
		AccessedAt: now,
		GalleryID:  &galleryID,
	}
	db.InsertImage(img)

	// Create file on disk
	fs.Save("todel", "original", []byte("data"))

	req := httptest.NewRequest("DELETE", "/gallery/del4/todel", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["deleted"] != "todel" {
		t.Errorf("deleted = %q, want 'todel'", resp["deleted"])
	}

	// Verify image is gone from DB
	got, _ := db.GetImageBySlug("todel")
	if got != nil {
		t.Error("image still exists in DB after delete")
	}

	// Verify file is gone from disk
	if fs.Exists("todel") {
		t.Error("files still exist on disk after delete")
	}
}

func TestGalleryHandler_GenerateUniqueSlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	slug := h.db.GenerateUniqueSlug("galleries", 4)
	if len(slug) != 4 {
		t.Errorf("slug length = %d, want 4", len(slug))
	}
}
