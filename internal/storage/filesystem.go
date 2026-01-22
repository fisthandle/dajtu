package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Filesystem struct {
	baseDir string
}

var mimeToExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/avif": ".avif",
}

var originalExts = []string{".jpg", ".png", ".gif", ".webp", ".avif"}

func NewFilesystem(baseDir string) (*Filesystem, error) {
	imagesDir := filepath.Join(baseDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return nil, fmt.Errorf("create images dir: %w", err)
	}
	return &Filesystem{baseDir: imagesDir}, nil
}

func GenerateSlug(length int) string {
	b := make([]byte, length/2+1)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
	}
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

func (fs *Filesystem) SaveOriginal(slug, name string, data []byte, mimeType string) (int64, error) {
	ext, ok := mimeToExt[mimeType]
	if !ok {
		return 0, fmt.Errorf("unsupported mime type: %s", mimeType)
	}

	dir := fs.DirPath(slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("create dir: %w", err)
	}

	name = sanitizeOriginalName(name)
	filename := "orig_" + name + ext
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("write file: %w", err)
	}
	return int64(len(data)), nil
}

func (fs *Filesystem) GetOriginalPath(slug, name string) (string, error) {
	dir := fs.DirPath(slug)
	name = sanitizeOriginalName(name)

	for _, ext := range originalExts {
		path := filepath.Join(dir, "orig_"+name+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func sanitizeOriginalName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" {
		return "original"
	}
	return name
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

func (fs *Filesystem) SaveBackup(slug string) error {
	srcPath := fs.Path(slug, "original")
	dstPath := filepath.Join(fs.DirPath(slug), "backup.webp")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0644)
}

func (fs *Filesystem) RestoreFromBackup(slug string) error {
	srcPath := filepath.Join(fs.DirPath(slug), "backup.webp")
	dstPath := fs.Path(slug, "original")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0644)
}

func (fs *Filesystem) HasBackup(slug string) bool {
	path := filepath.Join(fs.DirPath(slug), "backup.webp")
	_, err := os.Stat(path)
	return err == nil
}

func (fs *Filesystem) ReadBackup(slug string) ([]byte, error) {
	path := filepath.Join(fs.DirPath(slug), "backup.webp")
	return os.ReadFile(path)
}
