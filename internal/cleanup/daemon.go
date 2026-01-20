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
