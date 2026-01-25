package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"dajtu/internal/storage"
	"dajtu/internal/testutil"
)

func testAdminSetup(t *testing.T) (*storage.DB, *storage.Filesystem, *AdminHandler) {
	t.Helper()
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)
	return db, fs, NewAdminHandler(db, fs)
}

func seedAdminData(t *testing.T, db *storage.DB) (*storage.User, *storage.Gallery, *storage.Image) {
	t.Helper()

	now := time.Now().Unix()
	user := &storage.User{
		Slug:        "u001",
		DisplayName: "User One",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	userID, err := db.InsertUser(user)
	if err != nil {
		t.Fatalf("InsertUser() error = %v", err)
	}
	user.ID = userID

	gallery := &storage.Gallery{
		Slug:      "g001",
		EditToken: "token",
		Title:     "Test Gallery",
		UserID:    &userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, err := db.InsertGallery(gallery)
	if err != nil {
		t.Fatalf("InsertGallery() error = %v", err)
	}
	gallery.ID = galleryID

	image := &storage.Image{
		Slug:         "img01",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     1024,
		Width:        1,
		Height:       1,
		UserID:       &userID,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
		Downloads:    2,
		GalleryID:    &galleryID,
	}
	imageID, err := db.InsertImage(image)
	if err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}
	image.ID = imageID

	return user, gallery, image
}

func TestAdminHandler_Dashboard(t *testing.T) {
	db, _, h := testAdminSetup(t)
	seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin", nil)
	rec := httptest.NewRecorder()

	h.Dashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Dashboard") {
		t.Errorf("expected dashboard content")
	}
}

func TestAdminHandler_Users(t *testing.T) {
	db, _, h := testAdminSetup(t)
	user, _, _ := seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin/users", nil)
	rec := httptest.NewRecorder()

	h.Users(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, user.DisplayName) {
		t.Errorf("expected user display name in response")
	}
}

func TestAdminHandler_UserDetail(t *testing.T) {
	db, _, h := testAdminSetup(t)
	user, _, image := seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin/users/"+user.Slug, nil)
	req.SetPathValue("slug", user.Slug)
	rec := httptest.NewRecorder()

	h.UserDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, user.DisplayName) {
		t.Errorf("expected user display name in response")
	}
	if !strings.Contains(body, image.OriginalName) {
		t.Errorf("expected image name in response")
	}
}

func TestAdminHandler_Galleries(t *testing.T) {
	db, _, h := testAdminSetup(t)
	_, gallery, _ := seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin/galleries", nil)
	rec := httptest.NewRecorder()

	h.Galleries(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), gallery.Slug) {
		t.Errorf("expected gallery slug in response")
	}
}

func TestAdminHandler_GalleryDetail(t *testing.T) {
	db, _, h := testAdminSetup(t)
	_, gallery, image := seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin/galleries/"+gallery.Slug, nil)
	req.SetPathValue("slug", gallery.Slug)
	rec := httptest.NewRecorder()

	h.GalleryDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, gallery.Slug) {
		t.Errorf("expected gallery slug in response")
	}
	if !strings.Contains(body, image.OriginalName) {
		t.Errorf("expected image name in response")
	}
}

func TestAdminHandler_Images(t *testing.T) {
	db, _, h := testAdminSetup(t)
	_, _, image := seedAdminData(t, db)

	req := httptest.NewRequest("GET", "/admin/images", nil)
	rec := httptest.NewRecorder()

	h.Images(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), image.OriginalName) {
		t.Errorf("expected image name in response")
	}
}

func TestAdminHandler_DeleteGallery(t *testing.T) {
	db, fs, h := testAdminSetup(t)
	_, gallery, image := seedAdminData(t, db)

	if err := fs.Save(image.Slug, "original", []byte("data")); err != nil {
		t.Fatalf("save image: %v", err)
	}

	req := httptest.NewRequest("POST", "/admin/galleries/delete", nil)
	req.SetPathValue("id", int64ToString(t, gallery.ID))
	rec := httptest.NewRecorder()

	h.DeleteGallery(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, _ := db.GetGalleryByID(gallery.ID); got != nil {
		t.Fatalf("gallery still exists after delete")
	}
	if fs.Exists(image.Slug) {
		t.Errorf("image files still exist after delete")
	}
}

func TestAdminHandler_DeleteImage(t *testing.T) {
	db, fs, h := testAdminSetup(t)
	_, _, image := seedAdminData(t, db)

	if err := fs.Save(image.Slug, "original", []byte("data")); err != nil {
		t.Fatalf("save image: %v", err)
	}

	req := httptest.NewRequest("POST", "/admin/images/delete", nil)
	req.SetPathValue("id", int64ToString(t, image.ID))
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, _ := db.GetImageByID(image.ID); got != nil {
		t.Fatalf("image still exists after delete")
	}
	if fs.Exists(image.Slug) {
		t.Errorf("image files still exist after delete")
	}
}

func int64ToString(t *testing.T, v int64) string {
	t.Helper()
	return strconv.FormatInt(v, 10)
}
