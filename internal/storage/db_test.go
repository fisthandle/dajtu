package storage

import (
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewDB(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	if db.conn == nil {
		t.Error("db.conn is nil")
	}
}

func TestDB_InsertImage(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	img := &Image{
		Slug:         "abc12",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     12345,
		Width:        800,
		Height:       600,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	id, err := db.InsertImage(img)
	if err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertImage() id = %d, want > 0", id)
	}
}

func TestDB_InsertImage_DuplicateSlug(t *testing.T) {
	db := testDB(t)

	img := &Image{
		Slug:       "dup11",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}

	_, err := db.InsertImage(img)
	if err != nil {
		t.Fatalf("first InsertImage() error = %v", err)
	}

	_, err = db.InsertImage(img)
	if err == nil {
		t.Error("expected error on duplicate slug")
	}
}

func TestDB_GetImageBySlug(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	img := &Image{
		Slug:         "get11",
		OriginalName: "test.png",
		MimeType:     "image/png",
		FileSize:     9999,
		Width:        1024,
		Height:       768,
		CreatedAt:    now,
		AccessedAt:   now,
	}
	db.InsertImage(img)

	got, err := db.GetImageBySlug("get11")
	if err != nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetImageBySlug() = nil")
	}

	if got.Slug != "get11" {
		t.Errorf("Slug = %q, want %q", got.Slug, "get11")
	}
	if got.OriginalName != "test.png" {
		t.Errorf("OriginalName = %q, want %q", got.OriginalName, "test.png")
	}
	if got.FileSize != 9999 {
		t.Errorf("FileSize = %d, want %d", got.FileSize, 9999)
	}
}

func TestDB_GetImageBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetImageBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetImageBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_TouchImageBySlug(t *testing.T) {
	db := testDB(t)

	oldTime := time.Now().Add(-24 * time.Hour).Unix()
	img := &Image{
		Slug:       "touch",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  oldTime,
		AccessedAt: oldTime,
	}
	db.InsertImage(img)

	err := db.TouchImageBySlug("touch")
	if err != nil {
		t.Fatalf("TouchImageBySlug() error = %v", err)
	}

	got, _ := db.GetImageBySlug("touch")
	if got.AccessedAt <= oldTime {
		t.Error("AccessedAt was not updated")
	}
}

func TestDB_InsertGallery(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:        "gal1",
		EditToken:   "token123456789012345678901234",
		Title:       "My Gallery",
		Description: "Test description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := db.InsertGallery(g)
	if err != nil {
		t.Fatalf("InsertGallery() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertGallery() id = %d, want > 0", id)
	}
}

func TestDB_GetGalleryBySlug(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:        "getg",
		EditToken:   "edittoken123456789012345678901",
		Title:       "Test Gallery",
		Description: "Desc",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	got, err := db.GetGalleryBySlug("getg")
	if err != nil {
		t.Fatalf("GetGalleryBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetGalleryBySlug() = nil")
	}

	if got.Title != "Test Gallery" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Gallery")
	}
	if got.EditToken != "edittoken123456789012345678901" {
		t.Errorf("EditToken = %q, want %q", got.EditToken, "edittoken123456789012345678901")
	}
}

func TestDB_GetGalleryBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetGalleryBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetGalleryBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetGalleryBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_InsertUser(t *testing.T) {
	db := testDB(t)

	u := &User{
		Slug:        "user01",
		DisplayName: "Test User",
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	id, err := db.InsertUser(u)
	if err != nil {
		t.Fatalf("InsertUser() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertUser() id = %d, want > 0", id)
	}
}

func TestDB_GetUserBySlug(t *testing.T) {
	db := testDB(t)

	u := &User{
		Slug:        "getu01",
		DisplayName: "Found User",
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	db.InsertUser(u)

	got, err := db.GetUserBySlug("getu01")
	if err != nil {
		t.Fatalf("GetUserBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetUserBySlug() = nil")
	}
	if got.DisplayName != "Found User" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Found User")
	}
}

func TestDB_GetUserBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetUserBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetUserBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetUserBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_GetOrCreateBratUser_New(t *testing.T) {
	db := testDB(t)

	user, err := db.GetOrCreateBratUser("testuser", 1000)
	if err != nil {
		t.Fatalf("GetOrCreateBratUser failed: %v", err)
	}
	if user.DisplayName != "testuser" {
		t.Errorf("DisplayName = %q, want %q", user.DisplayName, "testuser")
	}
	if user.BratPunktacja != 1000 {
		t.Errorf("BratPunktacja = %d, want %d", user.BratPunktacja, 1000)
	}
	if user.Slug == "" {
		t.Error("Slug should not be empty")
	}
}

func TestDB_GetOrCreateBratUser_Existing(t *testing.T) {
	db := testDB(t)

	user1, _ := db.GetOrCreateBratUser("testuser", 500)

	user2, err := db.GetOrCreateBratUser("testuser", 1500)
	if err != nil {
		t.Fatalf("GetOrCreateBratUser failed: %v", err)
	}

	if user2.ID != user1.ID {
		t.Errorf("should return same user, got ID %d, want %d", user2.ID, user1.ID)
	}
	if user2.BratPunktacja != 1500 {
		t.Errorf("BratPunktacja should update, got %d, want %d", user2.BratPunktacja, 1500)
	}
}

func TestDB_GetGalleryImages(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "gimgs",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	// Add images to gallery
	for i := 0; i < 3; i++ {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   1000,
			CreatedAt:  now + int64(i),
			AccessedAt: now,
			GalleryID:  &galleryID,
		}
		db.InsertImage(img)
	}

	// Add image NOT in gallery
	img := &Image{
		Slug:       GenerateSlug(5),
		MimeType:   "image/jpeg",
		FileSize:   1000,
		CreatedAt:  now,
		AccessedAt: now,
	}
	db.InsertImage(img)

	images, err := db.GetGalleryImages(galleryID)
	if err != nil {
		t.Fatalf("GetGalleryImages() error = %v", err)
	}
	if len(images) != 3 {
		t.Errorf("GetGalleryImages() count = %d, want 3", len(images))
	}
}

func TestDB_GetGalleryImages_Empty(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "empty",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	images, err := db.GetGalleryImages(galleryID)
	if err != nil {
		t.Fatalf("GetGalleryImages() error = %v", err)
	}
	if len(images) != 0 {
		t.Errorf("GetGalleryImages(empty) count = %d, want 0", len(images))
	}
}

func TestDB_DeleteImageBySlug(t *testing.T) {
	db := testDB(t)

	img := &Image{
		Slug:       "del01",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}
	db.InsertImage(img)

	err := db.DeleteImageBySlug("del01")
	if err != nil {
		t.Fatalf("DeleteImageBySlug() error = %v", err)
	}

	got, _ := db.GetImageBySlug("del01")
	if got != nil {
		t.Error("image still exists after delete")
	}
}

func TestDB_DeleteImageBySlug_NonExistent(t *testing.T) {
	db := testDB(t)

	// Should not error
	err := db.DeleteImageBySlug("nonexistent")
	if err != nil {
		t.Errorf("DeleteImageBySlug(nonexistent) error = %v", err)
	}
}

func TestDB_GetOldestImages(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()

	// Insert images with different creation times
	for i := 0; i < 10; i++ {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   1000,
			CreatedAt:  now - int64(i*100), // Older images have lower timestamps
			AccessedAt: now,
		}
		db.InsertImage(img)
	}

	oldest, err := db.GetOldestImages(5)
	if err != nil {
		t.Fatalf("GetOldestImages() error = %v", err)
	}
	if len(oldest) != 5 {
		t.Errorf("GetOldestImages(5) count = %d, want 5", len(oldest))
	}

	// Verify sorted by created_at ASC
	for i := 1; i < len(oldest); i++ {
		if oldest[i].CreatedAt < oldest[i-1].CreatedAt {
			t.Error("images not sorted by created_at ASC")
		}
	}
}

func TestDB_GetTotalSize(t *testing.T) {
	db := testDB(t)

	// Empty DB
	size, err := db.GetTotalSize()
	if err != nil {
		t.Fatalf("GetTotalSize() error = %v", err)
	}
	if size != 0 {
		t.Errorf("GetTotalSize() on empty = %d, want 0", size)
	}

	// Add images
	now := time.Now().Unix()
	for _, fileSize := range []int64{1000, 2000, 3000} {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   fileSize,
			CreatedAt:  now,
			AccessedAt: now,
		}
		db.InsertImage(img)
	}

	size, err = db.GetTotalSize()
	if err != nil {
		t.Fatalf("GetTotalSize() error = %v", err)
	}
	if size != 6000 {
		t.Errorf("GetTotalSize() = %d, want 6000", size)
	}
}

func TestDB_SlugExists(t *testing.T) {
	db := testDB(t)

	// Image slug
	img := &Image{
		Slug:       "exist",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}
	db.InsertImage(img)

	exists, err := db.SlugExists("images", "exist")
	if err != nil {
		t.Fatalf("SlugExists() error = %v", err)
	}
	if !exists {
		t.Error("SlugExists() = false for existing slug")
	}

	exists, _ = db.SlugExists("images", "nonexistent")
	if exists {
		t.Error("SlugExists() = true for non-existing slug")
	}
}

func TestDB_SlugExists_Gallery(t *testing.T) {
	db := testDB(t)

	g := &Gallery{
		Slug:      "gexst",
		EditToken: "token",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	db.InsertGallery(g)

	exists, err := db.SlugExists("galleries", "gexst")
	if err != nil {
		t.Fatalf("SlugExists() error = %v", err)
	}
	if !exists {
		t.Error("SlugExists(galleries) = false for existing slug")
	}
}

func TestDB_ImageWithGallery_Cascade(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "casc",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	img := &Image{
		Slug:       "cascimg",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  now,
		AccessedAt: now,
		GalleryID:  &galleryID,
	}
	db.InsertImage(img)

	// Delete gallery - should cascade delete images
	_, err := db.conn.Exec("DELETE FROM galleries WHERE slug = ?", "casc")
	if err != nil {
		t.Fatalf("delete gallery error = %v", err)
	}

	// Image should be gone
	got, _ := db.GetImageBySlug("cascimg")
	if got != nil {
		t.Error("image still exists after gallery cascade delete")
	}
}

func TestDB_Close(t *testing.T) {
	dir := t.TempDir()
	db, _ := NewDB(dir)

	err := db.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestDB_CreateSession(t *testing.T) {
	db := testDB(t)

	// Create user first
	user, err := db.GetOrCreateBratUser("testuser", 100)
	if err != nil {
		t.Fatalf("GetOrCreateBratUser() error = %v", err)
	}

	session, err := db.CreateSession(user.ID, 30)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.Token == "" {
		t.Error("session.Token is empty")
	}
	if len(session.Token) != 64 {
		t.Errorf("session.Token length = %d, want 64", len(session.Token))
	}
	if session.UserID != user.ID {
		t.Errorf("session.UserID = %d, want %d", session.UserID, user.ID)
	}
	if session.ExpiresAt <= time.Now().Unix() {
		t.Error("session.ExpiresAt should be in the future")
	}
}

func TestDB_GetSession(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)
	session, _ := db.CreateSession(user.ID, 30)

	got, err := db.GetSession(session.Token)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSession() returned nil")
	}
	if got.UserID != user.ID {
		t.Errorf("GetSession().UserID = %d, want %d", got.UserID, user.ID)
	}
}

func TestDB_GetSession_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetSession("nonexistent_token")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got != nil {
		t.Error("GetSession() should return nil for nonexistent token")
	}
}

func TestDB_GetSession_Expired(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)

	// Insert expired session directly (expires_at in the past)
	expiredToken := "expired_test_token"
	_, err := db.conn.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		hashSessionToken(expiredToken), user.ID, time.Now().Unix()-3600, time.Now().Unix()-7200,
	)
	if err != nil {
		t.Fatalf("insert expired session error = %v", err)
	}

	got, err := db.GetSession(expiredToken)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got != nil {
		t.Error("GetSession() should return nil for expired session")
	}
}

func TestDB_DeleteSession(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)
	session, _ := db.CreateSession(user.ID, 30)

	err := db.DeleteSession(session.Token)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	got, _ := db.GetSession(session.Token)
	if got != nil {
		t.Error("session still exists after delete")
	}
}

func TestDB_DeleteUserSessions(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)

	// Create multiple sessions
	s1, _ := db.CreateSession(user.ID, 30)
	s2, _ := db.CreateSession(user.ID, 30)

	err := db.DeleteUserSessions(user.ID)
	if err != nil {
		t.Fatalf("DeleteUserSessions() error = %v", err)
	}

	got1, _ := db.GetSession(s1.Token)
	got2, _ := db.GetSession(s2.Token)
	if got1 != nil || got2 != nil {
		t.Error("sessions still exist after DeleteUserSessions")
	}
}

func TestDB_CleanExpiredSessions(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)

	// Insert expired session directly (expires_at in the past)
	_, err := db.conn.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		"expired_token_hash", user.ID, time.Now().Unix()-3600, time.Now().Unix()-7200,
	)
	if err != nil {
		t.Fatalf("insert expired session error = %v", err)
	}

	deleted, err := db.CleanExpiredSessions()
	if err != nil {
		t.Fatalf("CleanExpiredSessions() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("CleanExpiredSessions() deleted = %d, want 1", deleted)
	}
}

func TestDB_GetUserByID(t *testing.T) {
	db := testDB(t)

	user, _ := db.GetOrCreateBratUser("testuser", 100)

	got, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetUserByID() returned nil")
	}
	if got.Slug != user.Slug {
		t.Errorf("GetUserByID().Slug = %s, want %s", got.Slug, user.Slug)
	}
}

func TestDB_GetUserByID_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetUserByID(99999)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if got != nil {
		t.Error("GetUserByID() should return nil for nonexistent user")
	}
}
