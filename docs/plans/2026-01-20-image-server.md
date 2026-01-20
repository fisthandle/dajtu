# dajtu.com - Image Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bezpieczny serwer obrazków z auto-resize, cleanup i edytowalnymi galeriami.

**Architecture:** Go HTTP server + SQLite + filesystem storage. Caddy jako reverse proxy (SSL, cache, static files). Upload → walidacja magic bytes → re-encode przez libvips → zapis w strukturze hash-based. Cleanup daemon usuwa najstarsze gdy >49.5GB.

**Tech Stack:** Go 1.21+, bimg (libvips), SQLite3, Caddy, Docker

---

## Struktura projektu

```
/home/pawel/dev/dajtu/
├── cmd/
│   └── dajtu/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── storage/
│   │   ├── db.go
│   │   └── filesystem.go
│   ├── image/
│   │   ├── processor.go
│   │   └── validator.go
│   ├── handler/
│   │   ├── upload.go
│   │   ├── gallery.go
│   │   └── health.go
│   ├── cleanup/
│   │   └── daemon.go
│   └── middleware/
│       └── ratelimit.go
├── web/
│   └── templates/
│       └── gallery.html
├── docker/
│   ├── Dockerfile
│   └── Caddyfile
├── docker-compose.yml
├── docker-compose.prod.yml
├── go.mod
├── go.sum
└── docs/
    └── plans/
```

## Baza danych (SQLite)

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug CHAR(6) NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE galleries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug CHAR(4) NOT NULL UNIQUE,
    edit_token CHAR(32) NOT NULL,
    title TEXT,
    description TEXT,
    user_id INTEGER,               -- NULL = anonymous (v1)
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug CHAR(5) NOT NULL UNIQUE,
    original_name TEXT,
    mime_type TEXT NOT NULL,
    file_size INTEGER NOT NULL,    -- bytes (po konwersji)
    width INTEGER,
    height INTEGER,
    user_id INTEGER,               -- NULL = anonymous (v1)
    created_at INTEGER NOT NULL,   -- unix timestamp
    accessed_at INTEGER NOT NULL,  -- dla LRU cleanup
    gallery_id INTEGER,            -- NULL = standalone
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (gallery_id) REFERENCES galleries(id) ON DELETE CASCADE
);

CREATE INDEX idx_users_slug ON users(slug);
CREATE INDEX idx_images_created ON images(created_at);
CREATE INDEX idx_images_gallery ON images(gallery_id);
CREATE INDEX idx_images_user ON images(user_id);
CREATE INDEX idx_images_slug ON images(slug);
CREATE INDEX idx_galleries_slug ON galleries(slug);
CREATE INDEX idx_galleries_user ON galleries(user_id);
CREATE INDEX idx_galleries_edit ON galleries(edit_token);
```

## API Endpoints

```
POST /upload              → {"slug": "ab1c2", "url": "https://dajtu.com/i/ab1c2.webp", "sizes": {...}}
POST /gallery             → {"slug": "x7z9", "url": "https://dajtu.com/g/x7z9", "edit_token": "secret...", "images": [...]}
POST /gallery/:slug/add   → (edit_token required) dodaje obrazki
DELETE /gallery/:slug/:img_slug → (edit_token required) usuwa obrazek
GET /health               → {"status": "ok", "disk_usage_gb": 23.5}

# Static (served by Caddy):
GET /i/:slug.webp         → original (5-char slug)
GET /i/:slug/1920.webp    → 1920px width
GET /i/:slug/800.webp     → 800px
GET /i/:slug/200.webp     → thumbnail
GET /g/:slug              → gallery HTML page (4-char slug)
```

## Rozmiary obrazków

| Name | Max Width | Quality | Use Case |
|------|-----------|---------|----------|
| original | 4096px | 85% | full size |
| large | 1920px | 85% | desktop view |
| medium | 800px | 80% | mobile/embed |
| thumb | 200px | 75% | thumbnails |

---

## Task 1: Inicjalizacja projektu Go

**Files:**
- Create: `go.mod`
- Create: `cmd/dajtu/main.go`
- Create: `internal/config/config.go`

**Step 1: Inicjalizacja modułu Go**

```bash
cd /home/pawel/dev/dajtu
go mod init dajtu
```

**Step 2: Stwórz podstawową konfigurację**

`internal/config/config.go`:
```go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DataDir        string
	MaxFileSizeMB  int
	MaxDiskGB      float64
	CleanupTarget  float64 // cleanup do tego poziomu
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8080"),
		DataDir:       getEnv("DATA_DIR", "./data"),
		MaxFileSizeMB: getEnvInt("MAX_FILE_SIZE_MB", 20),
		MaxDiskGB:     getEnvFloat("MAX_DISK_GB", 50.0),
		CleanupTarget: getEnvFloat("CLEANUP_TARGET_GB", 45.0),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
```

**Step 3: Stwórz main.go ze skeleton serwerem**

`cmd/dajtu/main.go`:
```go
package main

import (
	"log"
	"net/http"

	"dajtu/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 4: Sprawdź czy kompiluje się**

```bash
cd /home/pawel/dev/dajtu
go build ./cmd/dajtu
```

Expected: binary `dajtu` utworzony bez błędów

**Step 5: Commit**

```bash
git add -A
git commit -m "feat(dajtu): init Go project with config and health endpoint"
```

---

## Task 2: SQLite storage layer

**Files:**
- Create: `internal/storage/db.go`
- Modify: `cmd/dajtu/main.go`
- Modify: `go.mod` (add sqlite dependency)

**Step 1: Dodaj zależność SQLite**

```bash
cd /home/pawel/dev/dajtu
go get github.com/mattn/go-sqlite3
```

**Step 2: Stwórz storage/db.go**

`internal/storage/db.go`:
```go
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type User struct {
	ID          int64
	Slug        string
	DisplayName string
	CreatedAt   int64
}

type Image struct {
	ID           int64
	Slug         string
	OriginalName string
	MimeType     string
	FileSize     int64
	Width        int
	Height       int
	UserID       *int64
	CreatedAt    int64
	AccessedAt   int64
	GalleryID    *int64
}

type Gallery struct {
	ID          int64
	Slug        string
	EditToken   string
	Title       string
	Description string
	UserID      *int64
	CreatedAt   int64
	UpdatedAt   int64
}

func NewDB(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "dajtu.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(6) NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS galleries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(4) NOT NULL UNIQUE,
		edit_token CHAR(32) NOT NULL,
		title TEXT,
		description TEXT,
		user_id INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug CHAR(5) NOT NULL UNIQUE,
		original_name TEXT,
		mime_type TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		width INTEGER,
		height INTEGER,
		user_id INTEGER,
		created_at INTEGER NOT NULL,
		accessed_at INTEGER NOT NULL,
		gallery_id INTEGER,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
		FOREIGN KEY (gallery_id) REFERENCES galleries(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_users_slug ON users(slug);
	CREATE INDEX IF NOT EXISTS idx_images_created ON images(created_at);
	CREATE INDEX IF NOT EXISTS idx_images_gallery ON images(gallery_id);
	CREATE INDEX IF NOT EXISTS idx_images_user ON images(user_id);
	CREATE INDEX IF NOT EXISTS idx_images_slug ON images(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_slug ON galleries(slug);
	CREATE INDEX IF NOT EXISTS idx_galleries_user ON galleries(user_id);
	CREATE INDEX IF NOT EXISTS idx_galleries_edit ON galleries(edit_token);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) InsertImage(img *Image) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO images (slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Slug, img.OriginalName, img.MimeType, img.FileSize, img.Width, img.Height, img.UserID, img.CreatedAt, img.AccessedAt, img.GalleryID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetImageBySlug(slug string) (*Image, error) {
	img := &Image{}
	err := db.conn.QueryRow(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images WHERE slug = ?`, slug).Scan(
		&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
		&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return img, err
}

func (db *DB) TouchImageBySlug(slug string) error {
	_, err := db.conn.Exec("UPDATE images SET accessed_at = ? WHERE slug = ?", time.Now().Unix(), slug)
	return err
}

func (db *DB) InsertGallery(g *Gallery) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO galleries (slug, edit_token, title, description, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.Slug, g.EditToken, g.Title, g.Description, g.UserID, g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetGalleryBySlug(slug string) (*Gallery, error) {
	g := &Gallery{}
	err := db.conn.QueryRow(`
		SELECT id, slug, edit_token, title, description, user_id, created_at, updated_at
		FROM galleries WHERE slug = ?`, slug).Scan(&g.ID, &g.Slug, &g.EditToken, &g.Title, &g.Description, &g.UserID, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

// User functions (for V2 SSO)
func (db *DB) InsertUser(u *User) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO users (slug, display_name, created_at)
		VALUES (?, ?, ?)`,
		u.Slug, u.DisplayName, u.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetUserBySlug(slug string) (*User, error) {
	u := &User{}
	err := db.conn.QueryRow(`
		SELECT id, slug, display_name, created_at
		FROM users WHERE slug = ?`, slug).Scan(&u.ID, &u.Slug, &u.DisplayName, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetGalleryImages(galleryID int64) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images WHERE gallery_id = ? ORDER BY created_at`, galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) DeleteImageBySlug(slug string) error {
	_, err := db.conn.Exec("DELETE FROM images WHERE slug = ?", slug)
	return err
}

func (db *DB) GetOldestImages(limit int) ([]*Image, error) {
	rows, err := db.conn.Query(`
		SELECT id, slug, original_name, mime_type, file_size, width, height, user_id, created_at, accessed_at, gallery_id
		FROM images ORDER BY created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.Slug, &img.OriginalName, &img.MimeType, &img.FileSize, &img.Width, &img.Height,
			&img.UserID, &img.CreatedAt, &img.AccessedAt, &img.GalleryID); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (db *DB) GetTotalSize() (int64, error) {
	var total int64
	err := db.conn.QueryRow("SELECT COALESCE(SUM(file_size), 0) FROM images").Scan(&total)
	return total, err
}

func (db *DB) SlugExists(table, slug string) (bool, error) {
	var count int
	err := db.conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE slug = ?", table), slug).Scan(&count)
	return count > 0, err
}

func (db *DB) Close() error {
	return db.conn.Close()
}
```

**Step 3: Zaktualizuj main.go żeby używał DB**

`cmd/dajtu/main.go`:
```go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

func main() {
	cfg := config.Load()

	db, err := storage.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		totalSize, _ := db.GetTotalSize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":        "ok",
			"disk_usage_gb": float64(totalSize) / (1024 * 1024 * 1024),
		})
	})

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 4: Sprawdź kompilację**

```bash
cd /home/pawel/dev/dajtu
go build ./cmd/dajtu
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add SQLite storage layer with images and galleries"
```

---

## Task 3: Image validator (magic bytes)

**Files:**
- Create: `internal/image/validator.go`

**Step 1: Stwórz validator.go**

`internal/image/validator.go`:
```go
package image

import (
	"bytes"
	"errors"
	"io"
)

var (
	ErrInvalidFormat = errors.New("invalid image format")
	ErrFileTooLarge  = errors.New("file too large")
)

type Format string

const (
	FormatJPEG Format = "image/jpeg"
	FormatPNG  Format = "image/png"
	FormatGIF  Format = "image/gif"
	FormatWebP Format = "image/webp"
	FormatAVIF Format = "image/avif"
)

var magicBytes = map[Format][]byte{
	FormatJPEG: {0xFF, 0xD8, 0xFF},
	FormatPNG:  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	FormatGIF:  {0x47, 0x49, 0x46, 0x38}, // GIF8
	FormatWebP: {0x52, 0x49, 0x46, 0x46}, // RIFF (need to check WEBP at offset 8)
	FormatAVIF: {0x00, 0x00, 0x00},       // ftyp box (need deeper check)
}

func ValidateAndDetect(r io.Reader, maxSize int64) (Format, []byte, error) {
	// Read entire file with size limit
	limited := io.LimitReader(r, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", nil, err
	}
	if int64(len(data)) > maxSize {
		return "", nil, ErrFileTooLarge
	}
	if len(data) < 12 {
		return "", nil, ErrInvalidFormat
	}

	format := detectFormat(data)
	if format == "" {
		return "", nil, ErrInvalidFormat
	}

	return format, data, nil
}

func detectFormat(data []byte) Format {
	// JPEG
	if bytes.HasPrefix(data, magicBytes[FormatJPEG]) {
		return FormatJPEG
	}

	// PNG
	if bytes.HasPrefix(data, magicBytes[FormatPNG]) {
		return FormatPNG
	}

	// GIF
	if bytes.HasPrefix(data, magicBytes[FormatGIF]) {
		return FormatGIF
	}

	// WebP: RIFF....WEBP
	if bytes.HasPrefix(data, magicBytes[FormatWebP]) && len(data) >= 12 {
		if bytes.Equal(data[8:12], []byte("WEBP")) {
			return FormatWebP
		}
	}

	// AVIF: ftyp box with avif/avis brand
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		brand := string(data[8:12])
		if brand == "avif" || brand == "avis" || brand == "mif1" {
			return FormatAVIF
		}
	}

	return ""
}
```

**Step 2: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add image format validator with magic bytes detection"
```

---

## Task 4: Image processor (bimg/libvips)

**Files:**
- Create: `internal/image/processor.go`
- Modify: `go.mod`

**Step 1: Dodaj bimg**

```bash
cd /home/pawel/dev/dajtu
go get github.com/h2non/bimg
```

Note: wymaga libvips-dev na systemie:
```bash
sudo apt-get install libvips-dev
```

**Step 2: Stwórz processor.go**

`internal/image/processor.go`:
```go
package image

import (
	"fmt"

	"github.com/h2non/bimg"
)

type Size struct {
	Name    string
	Width   int
	Quality int
}

var Sizes = []Size{
	{Name: "original", Width: 4096, Quality: 90},
	{Name: "1920", Width: 1920, Quality: 90},
	{Name: "800", Width: 800, Quality: 90},
	{Name: "200", Width: 200, Quality: 90},
}

type ProcessResult struct {
	Name   string
	Data   []byte
	Width  int
	Height int
}

func Process(data []byte) ([]ProcessResult, error) {
	img := bimg.NewImage(data)

	// Get original dimensions
	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("get size: %w", err)
	}

	var results []ProcessResult

	for _, s := range Sizes {
		// Skip if original is smaller than target
		targetWidth := s.Width
		if size.Width < targetWidth {
			targetWidth = size.Width
		}

		// Re-encode to WebP (this strips all metadata and potential malicious content)
		processed, err := bimg.NewImage(data).Process(bimg.Options{
			Width:         targetWidth,
			Type:          bimg.WEBP,
			Quality:       s.Quality,
			StripMetadata: true,
		})
		if err != nil {
			return nil, fmt.Errorf("process %s: %w", s.Name, err)
		}

		// Get resulting dimensions
		resultImg := bimg.NewImage(processed)
		resultSize, err := resultImg.Size()
		if err != nil {
			return nil, fmt.Errorf("get result size %s: %w", s.Name, err)
		}

		results = append(results, ProcessResult{
			Name:   s.Name,
			Data:   processed,
			Width:  resultSize.Width,
			Height: resultSize.Height,
		})

		// If original was smaller than first target, we only need one version
		if size.Width <= Sizes[0].Width && s.Name == "original" {
			// Still generate smaller sizes from this
			continue
		}
	}

	return results, nil
}
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add image processor with bimg/libvips for WebP conversion"
```

---

## Task 5: Filesystem storage

**Files:**
- Create: `internal/storage/filesystem.go`

**Step 1: Stwórz filesystem.go**

`internal/storage/filesystem.go`:
```go
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type Filesystem struct {
	baseDir string
}

func NewFilesystem(baseDir string) (*Filesystem, error) {
	imagesDir := filepath.Join(baseDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return nil, fmt.Errorf("create images dir: %w", err)
	}
	return &Filesystem{baseDir: imagesDir}, nil
}

func GenerateSlug(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// Path returns path for image: XX/slug/size.webp
// e.g., ab/ab1c2/original.webp (5-char slug)
func (fs *Filesystem) Path(slug, sizeName string) string {
	return filepath.Join(
		fs.baseDir,
		slug[0:2],
		slug,
		sizeName+".webp",
	)
}

func (fs *Filesystem) DirPath(slug string) string {
	return filepath.Join(
		fs.baseDir,
		slug[0:2],
		slug,
	)
}

func (fs *Filesystem) Save(slug, sizeName string, data []byte) error {
	path := fs.Path(slug, sizeName)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (fs *Filesystem) Delete(slug string) error {
	dir := fs.DirPath(slug)
	return os.RemoveAll(dir)
}

func (fs *Filesystem) Exists(slug string) bool {
	dir := fs.DirPath(slug)
	_, err := os.Stat(dir)
	return err == nil
}

func (fs *Filesystem) GetDiskUsage() (int64, error) {
	var total int64
	err := filepath.Walk(fs.baseDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
```

**Step 2: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add filesystem storage with hash-based directory structure"
```

---

## Task 6: Upload handler

**Files:**
- Create: `internal/handler/upload.go`
- Modify: `cmd/dajtu/main.go`

**Step 1: Stwórz upload.go**

`internal/handler/upload.go`:
```go
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/storage"
)

type UploadHandler struct {
	cfg *config.Config
	db  *storage.DB
	fs  *storage.Filesystem
}

func NewUploadHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *UploadHandler {
	return &UploadHandler{cfg: cfg, db: db, fs: fs}
}

type UploadResponse struct {
	Slug  string            `json:"slug"`
	URL   string            `json:"url"`
	Sizes map[string]string `json:"sizes"`
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024)

	// Parse multipart form
	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024); err != nil {
		jsonError(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "no file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate magic bytes and read content
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

	// Process image (re-encode + resize)
	results, err := image.Process(data)
	if err != nil {
		log.Printf("process error: %v", err)
		jsonError(w, "image processing failed", http.StatusInternalServerError)
		return
	}

	// Generate unique slug (5 chars for images)
	slug := h.generateUniqueSlug("images", 5)

	// Save all sizes
	var totalSize int64
	for _, res := range results {
		if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
			log.Printf("save error: %v", err)
			h.fs.Delete(slug) // cleanup on failure
			jsonError(w, "storage error", http.StatusInternalServerError)
			return
		}
		totalSize += int64(len(res.Data))
	}

	// Store metadata
	now := time.Now().Unix()
	originalResult := results[0]
	img := &storage.Image{
		Slug:         slug,
		OriginalName: header.Filename,
		MimeType:     string(format),
		FileSize:     totalSize,
		Width:        originalResult.Width,
		Height:       originalResult.Height,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	if _, err := h.db.InsertImage(img); err != nil {
		log.Printf("db error: %v", err)
		h.fs.Delete(slug)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Build response
	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", r.Host)
	}

	sizes := make(map[string]string)
	for _, res := range results {
		if res.Name == "original" {
			sizes[res.Name] = fmt.Sprintf("%s/i/%s.webp", baseURL, slug)
		} else {
			sizes[res.Name] = fmt.Sprintf("%s/i/%s/%s.webp", baseURL, slug, res.Name)
		}
	}

	resp := UploadResponse{
		Slug:  slug,
		URL:   sizes["original"],
		Sizes: sizes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UploadHandler) generateUniqueSlug(table string, length int) string {
	// Generate 20 candidates at once to minimize DB queries
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = storage.GenerateSlug(length)
	}

	for _, slug := range candidates {
		exists, _ := h.db.SlugExists(table, slug)
		if !exists {
			return slug
		}
	}
	// Fallback: try again (extremely unlikely to reach here)
	return h.generateUniqueSlug(table, length)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

**Step 2: Dodaj BaseURL do config**

Dodaj w `internal/config/config.go` do struct:
```go
BaseURL string
```

I w `Load()`:
```go
BaseURL: getEnv("BASE_URL", ""),
```

**Step 3: Zaktualizuj main.go**

`cmd/dajtu/main.go`:
```go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"dajtu/internal/config"
	"dajtu/internal/handler"
	"dajtu/internal/storage"
)

func main() {
	cfg := config.Load()

	db, err := storage.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFilesystem(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init filesystem: %v", err)
	}

	uploadHandler := handler.NewUploadHandler(cfg, db, fs)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		totalSize, _ := db.GetTotalSize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":        "ok",
			"disk_usage_gb": float64(totalSize) / (1024 * 1024 * 1024),
		})
	})
	mux.Handle("/upload", uploadHandler)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 4: Sprawdź kompilację**

```bash
cd /home/pawel/dev/dajtu
go build ./cmd/dajtu
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add upload handler with validation and processing"
```

---

## Task 7: Gallery handler

**Files:**
- Create: `internal/handler/gallery.go`
- Create: `web/templates/gallery.html`
- Modify: `cmd/dajtu/main.go`

**Step 1: Stwórz gallery.go**

`internal/handler/gallery.go`:
```go
package handler

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/image"
	"dajtu/internal/storage"
)

//go:embed templates/*
var templates embed.FS

type GalleryHandler struct {
	cfg      *config.Config
	db       *storage.DB
	fs       *storage.Filesystem
	template *template.Template
}

func NewGalleryHandler(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *GalleryHandler {
	tmpl := template.Must(template.ParseFS(templates, "templates/gallery.html"))
	return &GalleryHandler{cfg: cfg, db: db, fs: fs, template: tmpl}
}

type GalleryCreateResponse struct {
	Slug      string           `json:"slug"`
	URL       string           `json:"url"`
	EditToken string           `json:"edit_token"`
	Images    []UploadResponse `json:"images"`
}

// POST /gallery - create new gallery with images
func (h *GalleryHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10) // 10x for multiple files

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}

	// Create gallery (4-char slug)
	gallerySlug := h.generateUniqueSlug("galleries", 4)
	editToken := storage.GenerateSlug(32)
	now := time.Now().Unix()

	gallery := &storage.Gallery{
		Slug:        gallerySlug,
		EditToken:   editToken,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	galleryID, err := h.db.InsertGallery(gallery)
	if err != nil {
		log.Printf("db error: %v", err)
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	var uploadedImages []UploadResponse

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue // skip invalid files
		}

		results, err := image.Process(data)
		if err != nil {
			continue
		}

		slug := h.generateUniqueSlug("images", 5)

		var totalSize int64
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &galleryID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			if res.Name == "original" {
				sizes[res.Name] = baseURL + "/i/" + slug + ".webp"
			} else {
				sizes[res.Name] = baseURL + "/i/" + slug + "/" + res.Name + ".webp"
			}
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	if len(uploadedImages) == 0 {
		jsonError(w, "no valid images uploaded", http.StatusBadRequest)
		return
	}

	resp := GalleryCreateResponse{
		Slug:      gallerySlug,
		URL:       baseURL + "/g/" + gallerySlug,
		EditToken: editToken,
		Images:    uploadedImages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /gallery/:slug/add - add images to gallery
func (h *GalleryHandler) AddImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract gallery slug from path: /gallery/XXXX/add
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "add" {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		editToken = r.FormValue("edit_token")
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	if gallery.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	// Process same as Create but with existing gallery
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxFileSizeMB)*1024*1024*10)

	if err := r.ParseMultipartForm(int64(h.cfg.MaxFileSizeMB) * 1024 * 1024 * 10); err != nil {
		jsonError(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "no files provided", http.StatusBadRequest)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	now := time.Now().Unix()
	var uploadedImages []UploadResponse

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		maxSize := int64(h.cfg.MaxFileSizeMB) * 1024 * 1024
		format, data, err := image.ValidateAndDetect(file, maxSize)
		file.Close()
		if err != nil {
			continue
		}

		results, err := image.Process(data)
		if err != nil {
			continue
		}

		slug := h.generateUniqueSlug("images", 5)

		var totalSize int64
		for _, res := range results {
			if err := h.fs.Save(slug, res.Name, res.Data); err != nil {
				h.fs.Delete(slug)
				continue
			}
			totalSize += int64(len(res.Data))
		}

		img := &storage.Image{
			Slug:         slug,
			OriginalName: fileHeader.Filename,
			MimeType:     string(format),
			FileSize:     totalSize,
			Width:        results[0].Width,
			Height:       results[0].Height,
			CreatedAt:    now,
			AccessedAt:   now,
			GalleryID:    &gallery.ID,
		}

		if _, err := h.db.InsertImage(img); err != nil {
			h.fs.Delete(slug)
			continue
		}

		sizes := make(map[string]string)
		for _, res := range results {
			if res.Name == "original" {
				sizes[res.Name] = baseURL + "/i/" + slug + ".webp"
			} else {
				sizes[res.Name] = baseURL + "/i/" + slug + "/" + res.Name + ".webp"
			}
		}

		uploadedImages = append(uploadedImages, UploadResponse{
			Slug:  slug,
			URL:   sizes["original"],
			Sizes: sizes,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"added": uploadedImages,
	})
}

// DELETE /gallery/:slug/:img_slug - remove image from gallery
func (h *GalleryHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slugs from path: /gallery/XXXX/YYYYY
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	gallerySlug := parts[0]
	imageSlug := parts[1]

	// Verify edit token
	editToken := r.Header.Get("X-Edit-Token")
	if editToken == "" {
		r.ParseForm()
		editToken = r.FormValue("edit_token")
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	if gallery.EditToken != editToken {
		jsonError(w, "invalid edit token", http.StatusForbidden)
		return
	}

	// Verify image belongs to gallery
	img, err := h.db.GetImageBySlug(imageSlug)
	if err != nil || img == nil || img.GalleryID == nil || *img.GalleryID != gallery.ID {
		http.NotFound(w, r)
		return
	}

	// Delete from filesystem and database
	h.fs.Delete(imageSlug)
	h.db.DeleteImageBySlug(imageSlug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": imageSlug})
}

// GET /g/:slug - view gallery
func (h *GalleryHandler) View(w http.ResponseWriter, r *http.Request) {
	gallerySlug := strings.TrimPrefix(r.URL.Path, "/g/")
	if gallerySlug == "" {
		http.NotFound(w, r)
		return
	}

	gallery, err := h.db.GetGalleryBySlug(gallerySlug)
	if err != nil || gallery == nil {
		http.NotFound(w, r)
		return
	}

	images, err := h.db.GetGalleryImages(gallery.ID)
	if err != nil {
		http.Error(w, "error loading images", http.StatusInternalServerError)
		return
	}

	baseURL := h.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	type ImageData struct {
		Slug     string
		URL      string
		ThumbURL string
		Width    int
		Height   int
	}

	var imageData []ImageData
	for _, img := range images {
		imageData = append(imageData, ImageData{
			Slug:     img.Slug,
			URL:      baseURL + "/i/" + img.Slug + ".webp",
			ThumbURL: baseURL + "/i/" + img.Slug + "/200.webp",
			Width:    img.Width,
			Height:   img.Height,
		})
	}

	data := map[string]any{
		"Title":       gallery.Title,
		"Description": gallery.Description,
		"Images":      imageData,
		"BaseURL":     baseURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.template.Execute(w, data)
}

func (h *GalleryHandler) generateUniqueSlug(table string, length int) string {
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = storage.GenerateSlug(length)
	}

	for _, slug := range candidates {
		exists, _ := h.db.SlugExists(table, slug)
		if !exists {
			return slug
		}
	}
	return h.generateUniqueSlug(table, length)
}
```

**Step 2: Stwórz template**

Utwórz `internal/handler/templates/gallery.html`:
```html
<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{if .Title}}{{.Title}}{{else}}Galeria{{end}} - dajtu.com</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: system-ui, -apple-system, sans-serif;
            background: #111;
            color: #fff;
            min-height: 100vh;
        }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        h1 { margin-bottom: 20px; font-weight: 400; }
        .gallery {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 10px;
        }
        .gallery a {
            display: block;
            aspect-ratio: 1;
            overflow: hidden;
            border-radius: 4px;
        }
        .gallery img {
            width: 100%;
            height: 100%;
            object-fit: cover;
            transition: transform 0.2s;
        }
        .gallery a:hover img { transform: scale(1.05); }
        .lightbox {
            display: none;
            position: fixed;
            inset: 0;
            background: rgba(0,0,0,0.95);
            z-index: 1000;
            justify-content: center;
            align-items: center;
        }
        .lightbox.active { display: flex; }
        .lightbox img { max-width: 95vw; max-height: 95vh; }
        .lightbox-close {
            position: absolute;
            top: 20px;
            right: 20px;
            font-size: 30px;
            color: #fff;
            cursor: pointer;
        }
    </style>
</head>
<body>
    <div class="container">
        {{if .Title}}<h1>{{.Title}}</h1>{{end}}
        <div class="gallery">
            {{range .Images}}
            <a href="{{.URL}}" data-lightbox>
                <img src="{{.ThumbURL}}" alt="" loading="lazy">
            </a>
            {{end}}
        </div>
    </div>
    <div class="lightbox" id="lightbox">
        <span class="lightbox-close">&times;</span>
        <img src="" alt="">
    </div>
    <script>
        const lightbox = document.getElementById('lightbox');
        const lightboxImg = lightbox.querySelector('img');
        document.querySelectorAll('[data-lightbox]').forEach(a => {
            a.addEventListener('click', e => {
                e.preventDefault();
                lightboxImg.src = a.href;
                lightbox.classList.add('active');
            });
        });
        lightbox.addEventListener('click', () => lightbox.classList.remove('active'));
    </script>
</body>
</html>
```

**Step 3: Zaktualizuj main.go z routingiem galerii**

`cmd/dajtu/main.go`:
```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"dajtu/internal/config"
	"dajtu/internal/handler"
	"dajtu/internal/storage"
)

func main() {
	cfg := config.Load()

	db, err := storage.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFilesystem(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init filesystem: %v", err)
	}

	uploadHandler := handler.NewUploadHandler(cfg, db, fs)
	galleryHandler := handler.NewGalleryHandler(cfg, db, fs)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		totalSize, _ := db.GetTotalSize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":        "ok",
			"disk_usage_gb": float64(totalSize) / (1024 * 1024 * 1024),
		})
	})

	mux.Handle("/upload", uploadHandler)

	mux.HandleFunc("/gallery", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gallery" {
			galleryHandler.Create(w, r)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/gallery/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/gallery/")
		if strings.HasSuffix(path, "/add") {
			galleryHandler.AddImages(w, r)
		} else if r.Method == http.MethodDelete {
			galleryHandler.DeleteImage(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/g/", galleryHandler.View)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add gallery handler with create/add/delete/view"
```

---

## Task 8: Cleanup daemon

**Files:**
- Create: `internal/cleanup/daemon.go`
- Modify: `cmd/dajtu/main.go`

**Step 1: Stwórz daemon.go**

`internal/cleanup/daemon.go`:
```go
package cleanup

import (
	"log"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

type Daemon struct {
	cfg *config.Config
	db  *storage.DB
	fs  *storage.Filesystem
}

func NewDaemon(cfg *config.Config, db *storage.DB, fs *storage.Filesystem) *Daemon {
	return &Daemon{cfg: cfg, db: db, fs: fs}
}

func (d *Daemon) Start() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// Run immediately on start
		d.cleanup()

		for range ticker.C {
			d.cleanup()
		}
	}()
}

func (d *Daemon) cleanup() {
	totalSize, err := d.db.GetTotalSize()
	if err != nil {
		log.Printf("cleanup: failed to get total size: %v", err)
		return
	}

	maxBytes := int64(d.cfg.MaxDiskGB * 1024 * 1024 * 1024)
	targetBytes := int64(d.cfg.CleanupTarget * 1024 * 1024 * 1024)

	if totalSize < maxBytes {
		return
	}

	log.Printf("cleanup: disk usage %.2f GB exceeds %.2f GB, cleaning to %.2f GB",
		float64(totalSize)/(1024*1024*1024),
		d.cfg.MaxDiskGB,
		d.cfg.CleanupTarget)

	for totalSize > targetBytes {
		// Get oldest 100 images
		images, err := d.db.GetOldestImages(100)
		if err != nil {
			log.Printf("cleanup: failed to get oldest images: %v", err)
			return
		}

		if len(images) == 0 {
			break
		}

		for _, img := range images {
			if totalSize <= targetBytes {
				break
			}

			if err := d.fs.Delete(img.Slug); err != nil {
				log.Printf("cleanup: failed to delete files for %s: %v", img.Slug, err)
				continue
			}

			if err := d.db.DeleteImageBySlug(img.Slug); err != nil {
				log.Printf("cleanup: failed to delete db record for %s: %v", img.Slug, err)
				continue
			}

			totalSize -= img.FileSize
			log.Printf("cleanup: deleted %s (%.2f MB)", img.Slug, float64(img.FileSize)/(1024*1024))
		}
	}

	log.Printf("cleanup: done, current usage %.2f GB", float64(totalSize)/(1024*1024*1024))
}
```

**Step 2: Dodaj uruchomienie daemona w main.go**

Dodaj po inicjalizacji fs w `cmd/dajtu/main.go`:
```go
import "dajtu/internal/cleanup"

// po fs, err := ...
cleanupDaemon := cleanup.NewDaemon(cfg, db, fs)
cleanupDaemon.Start()
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add cleanup daemon for automatic disk space management"
```

---

## Task 9: Rate limiting middleware

**Files:**
- Create: `internal/middleware/ratelimit.go`
- Modify: `cmd/dajtu/main.go`

**Step 1: Stwórz ratelimit.go**

`internal/middleware/ratelimit.go`:
```go
package middleware

import (
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	count    int
	lastSeen time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}

	// Cleanup old entries every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > rl.window {
			delete(rl.visitors, ip)
		}
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
		return true
	}

	if time.Since(v.lastSeen) > rl.window {
		v.count = 1
		v.lastSeen = time.Now()
		return true
	}

	v.lastSeen = time.Now()
	v.count++
	return v.count <= rl.limit
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		if !rl.Allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

**Step 2: Dodaj rate limiting w main.go**

W `cmd/dajtu/main.go`, wrap upload handler:
```go
import "dajtu/internal/middleware"

// po galleryHandler := ...
uploadLimiter := middleware.NewRateLimiter(30, time.Minute) // 30 uploads/min

// zmień:
mux.Handle("/upload", uploadLimiter.Middleware(uploadHandler))
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add rate limiting middleware for upload protection"
```

---

## Task 10: Docker configuration

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `docker/Caddyfile`

**Step 1: Stwórz Dockerfile**

`Dockerfile`:
```dockerfile
FROM golang:1.21-bookworm AS builder

# Install libvips
RUN apt-get update && apt-get install -y libvips-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /dajtu ./cmd/dajtu

# Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y libvips42 ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /dajtu /usr/local/bin/dajtu

RUN useradd -r -s /bin/false dajtu
USER dajtu

EXPOSE 8080
CMD ["dajtu"]
```

**Step 2: Stwórz docker-compose.yml**

`docker-compose.yml`:
```yaml
services:
  app:
    build: .
    environment:
      - PORT=8080
      - DATA_DIR=/data
      - BASE_URL=http://localhost:8080
      - MAX_FILE_SIZE_MB=20
      - MAX_DISK_GB=50
      - CLEANUP_TARGET_GB=45
    volumes:
      - ./data:/data
    ports:
      - "8080:8080"
    restart: unless-stopped
```

**Step 3: Stwórz Caddyfile dla produkcji**

`docker/Caddyfile`:
```
dajtu.com {
    encode gzip

    # Static images - served by Caddy with caching
    handle /i/* {
        root * /data/images
        file_server
        header Cache-Control "public, max-age=31536000, immutable"

        # Rewrite /i/abc123.webp to /ab/c1/abc123/original.webp
        # Rewrite /i/abc123/800.webp to /ab/c1/abc123/800.webp
        @original path_regexp original ^/i/([a-f0-9]{2})([a-f0-9]{2})([a-f0-9]{8})\.webp$
        rewrite @original /{re.original.1}/{re.original.2}/{re.original.1}{re.original.2}{re.original.3}/original.webp

        @sized path_regexp sized ^/i/([a-f0-9]{2})([a-f0-9]{2})([a-f0-9]{8})/(\d+)\.webp$
        rewrite @sized /{re.sized.1}/{re.sized.2}/{re.sized.1}{re.sized.2}{re.sized.3}/{re.sized.4}.webp
    }

    # API and gallery pages - proxy to Go app
    handle {
        reverse_proxy app:8080
    }
}
```

**Step 4: Stwórz docker-compose.prod.yml**

`docker-compose.prod.yml`:
```yaml
services:
  app:
    build: .
    environment:
      - PORT=8080
      - DATA_DIR=/data
      - BASE_URL=https://dajtu.com
      - MAX_FILE_SIZE_MB=20
      - MAX_DISK_GB=50
      - CLEANUP_TARGET_GB=45
    volumes:
      - dajtu_data:/data
    networks:
      - dajtu
    restart: unless-stopped

  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./docker/Caddyfile:/etc/caddy/Caddyfile:ro
      - dajtu_data:/data:ro
      - caddy_data:/data
      - caddy_config:/config
    networks:
      - dajtu
    restart: unless-stopped
    depends_on:
      - app

volumes:
  dajtu_data:
  caddy_data:
  caddy_config:

networks:
  dajtu:
```

**Step 5: Stwórz .gitignore**

`.gitignore`:
```
/dajtu
/data/
*.db
*.db-wal
*.db-shm
```

**Step 6: Commit**

```bash
git add -A
git commit -m "feat(dajtu): add Docker configuration for dev and production"
```

---

## Task 11: Test lokalnie

**Step 1: Zainstaluj libvips**

```bash
sudo apt-get install libvips-dev
```

**Step 2: Zbuduj i uruchom**

```bash
cd /home/pawel/dev/dajtu
go build ./cmd/dajtu
./dajtu
```

**Step 3: Testuj upload**

```bash
# Single image
curl -X POST -F "file=@/path/to/test.jpg" http://localhost:8080/upload

# Gallery
curl -X POST -F "files=@test1.jpg" -F "files=@test2.jpg" -F "title=Test Gallery" http://localhost:8080/gallery
```

**Step 4: Sprawdź health**

```bash
curl http://localhost:8080/health
```

---

## Task 12: Test z Docker

**Step 1: Build i uruchom**

```bash
cd /home/pawel/dev/dajtu
docker compose up --build
```

**Step 2: Testuj jak w Task 11**

**Step 3: Commit wszystkie poprawki**

```bash
git add -A
git commit -m "fix(dajtu): fixes from local testing"
```

---

## V2 - SSO Auth (po walidacji V1)

> Do implementacji później - gdy V1 będzie działać stabilnie

**Endpoint:** `GET /auth/@data`

**Mechanizm:**
1. brazar.eu generuje zaszyfrowany payload (AES-256-CBC) z:
   - timestamp
   - user_id
   - pseudonim
   - HMAC hash
2. dajtu.com dekoduje, weryfikuje, tworzy sesję
3. Sesja cookie pozwala na upload/edycję galerii

**Config (dodać do .env):**
```
AUTH_ENABLED=false
AUTH_ENCRYPTION_KEY=
AUTH_ENCRYPTION_IV=
AUTH_HASH_SECRET=
AUTH_MAX_SKEW_SECONDS=300
```

---

## Podsumowanie bezpieczeństwa

| Wektor | Ochrona |
|--------|---------|
| Malicious file upload | Magic bytes + re-encoding przez libvips |
| Image bomb (decompression) | bimg limity + MaxFileSizeMB |
| Metadata injection | StripMetadata: true |
| Path traversal | Random ID, brak user input w ścieżkach |
| DoS | Rate limiting, MaxBytesReader |
| Brute force gallery | 32-char edit token |
| Disk exhaustion | Cleanup daemon |
