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
