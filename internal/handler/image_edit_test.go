package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"
	"dajtu/internal/testutil"
)

func testEditSetup(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, *ImageEditHandler) {
	t.Helper()
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)
	handler := NewImageEditHandler(db, fs, image.NewProcessor(), cfg)
	return cfg, db, fs, handler
}

func createEditMultipartRequest(t *testing.T, url, fieldName, filename string, content []byte, mode string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if mode != "" {
		if err := writer.WriteField("mode", mode); err != nil {
			t.Fatalf("write mode: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestImageViewHandler_NotFound(t *testing.T) {
	db, _ := testutil.TestDB(t)
	cfg := testutil.TestConfig(t)
	h := NewImageViewHandler(db, cfg)

	req := httptest.NewRequest("GET", "/i/missing", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestImageViewHandler_EditMode(t *testing.T) {
	db, _ := testutil.TestDB(t)
	cfg := testutil.TestConfig(t)
	h := NewImageViewHandler(db, cfg)

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         "img01",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     123,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
		EditToken:    "edit-token-1",
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/i/img01?edit=edit-token-1", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req, "img01")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Edit Token:") {
		t.Errorf("expected edit token section in response")
	}
	if !strings.Contains(body, "edit-token-1") {
		t.Errorf("expected edit token in response")
	}
}

func TestImageEditHandler_Unauthorized(t *testing.T) {
	_, db, _, h := testEditSetup(t)

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:       "unauth",
		MimeType:   "image/jpeg",
		FileSize:   10,
		CreatedAt:  now,
		UpdatedAt:  now,
		AccessedAt: now,
		EditToken:  "token-unauth",
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/i/unauth/edit", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req, "unauth")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestImageEditHandler_AuthorizedGet(t *testing.T) {
	_, db, _, h := testEditSetup(t)

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         "auth01",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     10,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
		EditToken:    "token-auth",
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/i/auth01/edit?edit=token-auth", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req, "auth01")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Zapisz jako") {
		t.Errorf("expected edit page content")
	}
}

func TestImageEditHandler_PostNew(t *testing.T) {
	if _, err := image.Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	_, db, fs, h := testEditSetup(t)

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:       "orig1",
		MimeType:   "image/jpeg",
		FileSize:   10,
		CreatedAt:  now,
		UpdatedAt:  now,
		AccessedAt: now,
		EditToken:  "token-new",
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	if err := fs.Save("orig1", "original", testutil.SampleJPEG()); err != nil {
		t.Fatalf("save original: %v", err)
	}

	req := createEditMultipartRequest(t, "/i/orig1/edit", "file", "edit.jpg", testutil.SampleJPEG(), "new")
	req.Header.Set("X-Edit-Token", "token-new")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req, "orig1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["slug"] == "" || resp["slug"] == "orig1" {
		t.Fatalf("unexpected new slug: %q", resp["slug"])
	}

	newImg, err := db.GetImageBySlug(resp["slug"])
	if err != nil || newImg == nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if !newImg.Edited {
		t.Errorf("new image should be marked edited")
	}
	if newImg.UserID != nil {
		t.Errorf("new image should not have user when edit token used")
	}
}

func TestImageEditHandler_RestoreOriginal(t *testing.T) {
	if _, err := image.Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	_, db, fs, h := testEditSetup(t)

	user, err := db.GetOrCreateBratUser("restoreuser")
	if err != nil {
		t.Fatalf("GetOrCreateBratUser() error = %v", err)
	}

	now := time.Now().Unix()
	img := &storage.Image{
		Slug:         "rest1",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     10,
		Width:        1,
		Height:       1,
		UserID:       &user.ID,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
		Edited:       true,
	}
	if _, err := db.InsertImage(img); err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}

	if err := fs.Save("rest1", "original", testutil.SampleJPEG()); err != nil {
		t.Fatalf("save original: %v", err)
	}
	if err := fs.SaveBackup("rest1"); err != nil {
		t.Fatalf("save backup: %v", err)
	}

	req := httptest.NewRequest("POST", "/i/rest1/restore", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rec := httptest.NewRecorder()

	h.RestoreOriginal(rec, req, "rest1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %q, want %q", resp["status"], "ok")
	}

	updated, err := db.GetImageBySlug("rest1")
	if err != nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if updated.Edited {
		t.Errorf("expected image to be unmarked as edited")
	}
}
