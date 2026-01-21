package storage

import (
	"os"
	"testing"
)

func TestAdminQueries(t *testing.T) {
	// Create temp dir for DB
	tmpDir, err := os.MkdirTemp("", "dajtu_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := NewDB(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Insert test user
	_, err = db.conn.Exec(`INSERT INTO users (slug, display_name, created_at) VALUES ('usr1', 'TestUser', 1000)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert test gallery
	_, err = db.conn.Exec(`INSERT INTO galleries (slug, edit_token, title, user_id, created_at, updated_at) VALUES ('gal1', 'token123', 'Test Gallery', 1, 1000, 1000)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert test image
	_, err = db.conn.Exec(`INSERT INTO images (slug, original_name, mime_type, file_size, downloads, user_id, gallery_id, created_at, accessed_at) VALUES ('img01', 'test.jpg', 'image/jpeg', 12345, 42, 1, 1, 1000, 2000)`)
	if err != nil {
		t.Fatal(err)
	}

	// Test GetStats
	stats, err := db.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalImages != 1 {
		t.Errorf("Expected 1 image, got %d", stats.TotalImages)
	}
	if stats.TotalUsers != 1 {
		t.Errorf("Expected 1 user, got %d", stats.TotalUsers)
	}
	if stats.TotalGalleries != 1 {
		t.Errorf("Expected 1 gallery, got %d", stats.TotalGalleries)
	}
	if stats.DiskUsageBytes != 12345 {
		t.Errorf("Expected 12345 bytes, got %d", stats.DiskUsageBytes)
	}

	// Test ListUsers
	users, err := db.ListUsers(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
	if users[0].DisplayName != "TestUser" {
		t.Errorf("Expected TestUser, got %s", users[0].DisplayName)
	}

	// Test ListGalleriesAdmin
	galleries, err := db.ListGalleriesAdmin(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(galleries) != 1 {
		t.Errorf("Expected 1 gallery, got %d", len(galleries))
	}
	if galleries[0].Title != "Test Gallery" {
		t.Errorf("Expected 'Test Gallery', got %s", galleries[0].Title)
	}
	if galleries[0].OwnerName != "TestUser" {
		t.Errorf("Expected OwnerName 'TestUser', got '%s'", galleries[0].OwnerName)
	}
	if galleries[0].ImageCount != 1 {
		t.Errorf("Expected 1 image in gallery, got %d", galleries[0].ImageCount)
	}

	// Test ListImagesAdmin
	images, err := db.ListImagesAdmin(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 {
		t.Errorf("Expected 1 image, got %d", len(images))
	}
	if images[0].OriginalName != "test.jpg" {
		t.Errorf("Expected 'test.jpg', got %s", images[0].OriginalName)
	}
	if images[0].Downloads != 42 {
		t.Errorf("Expected 42 downloads, got %d", images[0].Downloads)
	}
	if images[0].OwnerName != "TestUser" {
		t.Errorf("Expected OwnerName 'TestUser', got '%s'", images[0].OwnerName)
	}
	if images[0].GallerySlug != "gal1" {
		t.Errorf("Expected GallerySlug 'gal1', got '%s'", images[0].GallerySlug)
	}
}
