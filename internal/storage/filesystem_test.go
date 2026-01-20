package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFilesystem(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFilesystem(dir)
	if err != nil {
		t.Fatalf("NewFilesystem() error = %v", err)
	}
	if fs == nil {
		t.Fatal("NewFilesystem() returned nil")
	}

	// Check images directory was created
	imagesDir := filepath.Join(dir, "images")
	info, err := os.Stat(imagesDir)
	if err != nil {
		t.Fatalf("images directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("images path is not a directory")
	}
}

func TestNewFilesystem_InvalidPath(t *testing.T) {
	// Try to create in a read-only location
	_, err := NewFilesystem("/proc/test-readonly")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		length int
	}{
		{4},
		{5},
		{6},
		{32},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			slug := GenerateSlug(tt.length)
			if len(slug) != tt.length {
				t.Errorf("GenerateSlug(%d) length = %d, want %d", tt.length, len(slug), tt.length)
			}

			// Verify it's hex (lowercase a-f, 0-9)
			for _, c := range slug {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("invalid character in slug: %c", c)
				}
			}
		})
	}
}

func TestGenerateSlug_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		slug := GenerateSlug(6)
		if seen[slug] {
			t.Errorf("duplicate slug generated: %s", slug)
		}
		seen[slug] = true
	}
}

func TestFilesystem_Path(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	tests := []struct {
		slug     string
		sizeName string
		want     string
	}{
		{"ab1c2", "original", filepath.Join(dir, "images", "ab", "ab1c2", "original.webp")},
		{"ab1c2", "800", filepath.Join(dir, "images", "ab", "ab1c2", "800.webp")},
		{"xyz99", "200", filepath.Join(dir, "images", "xy", "xyz99", "200.webp")},
	}

	for _, tt := range tests {
		t.Run(tt.slug+"/"+tt.sizeName, func(t *testing.T) {
			got := fs.Path(tt.slug, tt.sizeName)
			if got != tt.want {
				t.Errorf("Path(%q, %q) = %q, want %q", tt.slug, tt.sizeName, got, tt.want)
			}
		})
	}
}

func TestFilesystem_DirPath(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "ab1c2"
	want := filepath.Join(dir, "images", "ab", "ab1c2")
	got := fs.DirPath(slug)
	if got != want {
		t.Errorf("DirPath(%q) = %q, want %q", slug, got, want)
	}
}

func TestFilesystem_Save(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "test1"
	sizeName := "original"
	data := []byte("test image data")

	err := fs.Save(slug, sizeName, data)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	path := fs.Path(slug, sizeName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("file content = %q, want %q", content, data)
	}
}

func TestFilesystem_Save_MultipleSizes(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "multi1"
	sizes := []string{"original", "1920", "800", "200"}

	for _, size := range sizes {
		data := []byte("data for " + size)
		if err := fs.Save(slug, size, data); err != nil {
			t.Errorf("Save(%q, %q) error = %v", slug, size, err)
		}
	}

	// Verify all files exist
	for _, size := range sizes {
		path := fs.Path(slug, size)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file not created for size %q", size)
		}
	}
}

func TestFilesystem_Delete(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "del01"
	fs.Save(slug, "original", []byte("data"))
	fs.Save(slug, "800", []byte("data"))

	err := fs.Delete(slug)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify directory is gone
	if fs.Exists(slug) {
		t.Error("slug directory still exists after Delete()")
	}
}

func TestFilesystem_Delete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	// Should not error on non-existent
	err := fs.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete(nonexistent) error = %v, want nil", err)
	}
}

func TestFilesystem_Exists(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "exist"

	if fs.Exists(slug) {
		t.Error("Exists() = true before Save()")
	}

	fs.Save(slug, "original", []byte("data"))

	if !fs.Exists(slug) {
		t.Error("Exists() = false after Save()")
	}

	fs.Delete(slug)

	if fs.Exists(slug) {
		t.Error("Exists() = true after Delete()")
	}
}

func TestFilesystem_GetDiskUsage(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	// Initially empty
	usage, err := fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("GetDiskUsage() error = %v", err)
	}
	if usage != 0 {
		t.Errorf("GetDiskUsage() on empty = %d, want 0", usage)
	}

	// Add some files
	fs.Save("test1", "original", []byte(strings.Repeat("x", 1000)))
	fs.Save("test2", "original", []byte(strings.Repeat("y", 500)))

	usage, err = fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("GetDiskUsage() error = %v", err)
	}
	if usage != 1500 {
		t.Errorf("GetDiskUsage() = %d, want 1500", usage)
	}
}

func TestFilesystem_GetDiskUsage_AfterDelete(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	fs.Save("test1", "original", []byte(strings.Repeat("x", 1000)))

	fs.Delete("test1")

	usage, _ := fs.GetDiskUsage()
	if usage != 0 {
		t.Errorf("GetDiskUsage() after delete = %d, want 0", usage)
	}
}
