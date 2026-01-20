package cleanup

import (
	"testing"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

func testSetup(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, func()) {
	t.Helper()
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.001,  // 1 MB
		CleanupTarget: 0.0005, // 0.5 MB
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

func TestNewDaemon(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	d := NewDaemon(cfg, db, fs)
	if d == nil {
		t.Fatal("NewDaemon() = nil")
	}
	if d.cfg != cfg {
		t.Error("daemon.cfg not set")
	}
	if d.db != db {
		t.Error("daemon.db not set")
	}
	if d.fs != fs {
		t.Error("daemon.fs not set")
	}
}

func TestDaemon_Cleanup_UnderLimit(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	// Add small amount of data
	now := time.Now().Unix()
	img := &storage.Image{
		Slug:       "small",
		MimeType:   "image/jpeg",
		FileSize:   100, // 100 bytes, well under 1MB limit
		CreatedAt:  now,
		AccessedAt: now,
	}
	db.InsertImage(img)
	fs.Save("small", "original", make([]byte, 100))

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Image should still exist
	got, _ := db.GetImageBySlug("small")
	if got == nil {
		t.Error("image should not be deleted when under limit")
	}
}

func TestDaemon_Cleanup_OverLimit(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.000001,  // ~1KB limit
		CleanupTarget: 0.0000005, // ~500 bytes target
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	// Add data over limit
	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		slug := storage.GenerateSlug(5)
		img := &storage.Image{
			Slug:       slug,
			MimeType:   "image/jpeg",
			FileSize:   200, // 200 bytes each
			CreatedAt:  now - int64(i*100),
			AccessedAt: now,
		}
		db.InsertImage(img)
		fs.Save(slug, "original", make([]byte, 200))
	}

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Some images should be deleted
	totalSize, _ := db.GetTotalSize()
	targetBytes := int64(cfg.CleanupTarget * 1024 * 1024 * 1024)

	if totalSize > targetBytes {
		t.Errorf("total size %d still over target %d after cleanup", totalSize, targetBytes)
	}
}

func TestDaemon_Cleanup_DeletesOldestFirst(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.000001,
		CleanupTarget: 0.0000003,
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	now := time.Now().Unix()

	// oldest image
	oldImg := &storage.Image{
		Slug:       "oldest",
		MimeType:   "image/jpeg",
		FileSize:   500,
		CreatedAt:  now - 1000, // Oldest
		AccessedAt: now,
	}
	db.InsertImage(oldImg)
	fs.Save("oldest", "original", make([]byte, 500))

	// newer image
	newImg := &storage.Image{
		Slug:       "newest",
		MimeType:   "image/jpeg",
		FileSize:   500,
		CreatedAt:  now, // Newest
		AccessedAt: now,
	}
	db.InsertImage(newImg)
	fs.Save("newest", "original", make([]byte, 500))

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Oldest should be deleted first
	old, _ := db.GetImageBySlug("oldest")
	new, _ := db.GetImageBySlug("newest")

	if old != nil && new == nil {
		t.Error("newer image deleted before older image")
	}
}

func TestDaemon_Cleanup_NoImages(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	// High usage configured but no images
	cfg.MaxDiskGB = 0.000000001

	d := NewDaemon(cfg, db, fs)

	// Should not panic
	d.cleanup()
}

func TestDaemon_Start(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	d := NewDaemon(cfg, db, fs)

	// Just verify Start doesn't panic
	d.Start()

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)
}
