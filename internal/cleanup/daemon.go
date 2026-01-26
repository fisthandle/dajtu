package cleanup

import (
	"os"
	"path/filepath"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/logging"
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
	d.cleanupCache()

	if deleted, err := d.db.CleanExpiredSessions(); err != nil {
		logging.Get("cleanup").Printf("cleanup: failed to clean sessions: %v", err)
	} else if deleted > 0 {
		logging.Get("cleanup").Printf("cleanup: deleted %d expired sessions", deleted)
	}

	totalSize, err := d.db.GetTotalSize()
	if err != nil {
		logging.Get("cleanup").Printf("cleanup: failed to get total size: %v", err)
		return
	}

	maxBytes := int64(d.cfg.MaxDiskGB * 1024 * 1024 * 1024)
	targetBytes := int64(d.cfg.CleanupTarget * 1024 * 1024 * 1024)

	if totalSize < maxBytes {
		return
	}

	logging.Get("cleanup").Printf("cleanup: disk usage %.2f GB exceeds %.2f GB, cleaning to %.2f GB",
		float64(totalSize)/(1024*1024*1024),
		d.cfg.MaxDiskGB,
		d.cfg.CleanupTarget)

	for totalSize > targetBytes {
		// Get oldest 100 images
		images, err := d.db.GetOldestImages(100)
		if err != nil {
			logging.Get("cleanup").Printf("cleanup: failed to get oldest images: %v", err)
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
				logging.Get("cleanup").Printf("cleanup: failed to delete files for %s: %v", img.Slug, err)
				continue
			}

			if err := d.db.DeleteImageBySlug(img.Slug); err != nil {
				logging.Get("cleanup").Printf("cleanup: failed to delete db record for %s: %v", img.Slug, err)
				continue
			}

			totalSize -= img.FileSize
			logging.Get("cleanup").Printf("cleanup: deleted %s (%.2f MB)", img.Slug, float64(img.FileSize)/(1024*1024))
		}
	}

	logging.Get("cleanup").Printf("cleanup: done, current usage %.2f GB", float64(totalSize)/(1024*1024*1024))
}

func (d *Daemon) cleanupCache() {
	if d.cfg.CacheDir == "" {
		return
	}
	if _, err := os.Stat(d.cfg.CacheDir); err != nil {
		if os.IsNotExist(err) {
			return
		}
		logging.Get("cleanup").Printf("cleanup: cache stat error: %v", err)
		return
	}

	cutoff := time.Now().Add(-48 * time.Hour)
	var removed int
	var bytes int64

	err := filepath.WalkDir(d.cfg.CacheDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed++
		bytes += info.Size()
		return nil
	})
	if err != nil {
		logging.Get("cleanup").Printf("cleanup: cache walk error: %v", err)
		return
	}
	if removed > 0 {
		logging.Get("cleanup").Printf("cleanup: cache removed %d files (%.2f MB)", removed, float64(bytes)/(1024*1024))
	}
}
