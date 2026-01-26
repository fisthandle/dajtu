package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dajtu/internal/cleanup"
	"dajtu/internal/config"
	"dajtu/internal/handler"
	"dajtu/internal/image"
	"dajtu/internal/logging"
	"dajtu/internal/middleware"
	"dajtu/internal/storage"

	"golang.org/x/sync/singleflight"
)

func isValidSlug(s string) bool {
	if len(s) < 2 || len(s) > 10 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

var resizeGroup singleflight.Group

var dynamicSizes = map[string]int{
	"800":  800,
	"1200": 1200,
	"1600": 1600,
	"2400": 2400,
}

type resizeResult struct {
	path string
	data []byte
}

func webpPath(dataDir, slug, name string) string {
	return filepath.Join(dataDir, "images", slug[:2], slug, name+".webp")
}

func cachePath(cacheDir, slug, size string) string {
	filename := slug + "_" + size + ".webp"
	return filepath.Join(cacheDir, slug[:2], filename)
}

func cacheValid(path string, originMod time.Time) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.ModTime().Before(originMod)
}

func touch(path string) {
	now := time.Now()
	_ = os.Chtimes(path, now, now)
}

func writeCacheFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func serveWebpFile(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/webp")
	http.ServeFile(w, r, path)
}

func serveWebpBytes(w http.ResponseWriter, data []byte) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/webp")
	w.Write(data)
}

func main() {
	cfg := config.Load()

	if err := logging.Init(cfg.LogDir); err != nil {
		log.Printf("Failed to init loggers: %v", err)
	}

	db, err := storage.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFilesystem(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init filesystem: %v", err)
	}

	cleanupDaemon := cleanup.NewDaemon(cfg, db, fs)
	cleanupDaemon.Start()

	processor := image.NewProcessor()

	uploadHandler := handler.NewUploadHandler(cfg, db, fs, processor)
	galleryHandler := handler.NewGalleryHandler(cfg, db, fs)
	authHandler, err := handler.NewAuthHandler(cfg, db)
	if err != nil {
		log.Fatalf("Failed to init SSO: %v", err)
	}
	userHandler := handler.NewUserHandler(cfg, db)
	uploadLimiter := middleware.NewRateLimiter(30, time.Minute)
	sessionMiddleware := middleware.NewSessionMiddleware(db)
	trafficStats := middleware.NewTrafficStats()
	adminHandler := handler.NewAdminHandler(cfg, db, fs, trafficStats)
	adminMiddleware := middleware.NewAdminMiddleware(cfg.AdminNicks)
	requestLogger := middleware.NewRequestLogger(trafficStats)

	bratUploadHandler := handler.NewBratUploadHandler(cfg, db, fs, authHandler.GetDecoder(), processor)

	imageViewHandler := handler.NewImageViewHandler(db, cfg)

	imageEditHandler := handler.NewImageEditHandler(db, fs, processor, cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("/", galleryHandler.Index)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		totalSize, _ := db.GetTotalSize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":        "ok",
			"disk_usage_gb": float64(totalSize) / (1024 * 1024 * 1024),
		})
	})

	mux.Handle("/upload", uploadLimiter.Middleware(uploadHandler))

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
		} else if strings.HasSuffix(path, "/title") {
			galleryHandler.UpdateTitle(w, r)
		} else if r.Method == http.MethodDelete {
			galleryHandler.DeleteImage(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/g/", galleryHandler.View)
	mux.HandleFunc("/u/", userHandler.View)
	mux.HandleFunc("/brrrt/", authHandler.HandleBratSSO)
	mux.HandleFunc("/logout", authHandler.Logout)
	mux.Handle("/brtup/", bratUploadHandler)

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin", adminHandler.Dashboard)
	adminMux.HandleFunc("GET /admin/users", adminHandler.Users)
	adminMux.HandleFunc("GET /admin/users/{slug}", adminHandler.UserDetail)
	adminMux.HandleFunc("GET /admin/galleries", adminHandler.Galleries)
	adminMux.HandleFunc("GET /admin/galleries/{slug}", adminHandler.GalleryDetail)
	adminMux.HandleFunc("POST /admin/galleries/{id}/delete", adminHandler.DeleteGallery)
	adminMux.HandleFunc("GET /admin/images", adminHandler.Images)
	adminMux.HandleFunc("GET /admin/logs", adminHandler.Logs)
	adminMux.HandleFunc("POST /admin/images/{id}/delete", adminHandler.DeleteImage)

	mux.Handle("/admin", adminMiddleware.Middleware(adminMux))
	mux.Handle("/admin/", adminMiddleware.Middleware(adminMux))

	mux.HandleFunc("/i/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/i/")
		parts := strings.Split(path, "/")

		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		if len(parts) > 2 {
			http.NotFound(w, r)
			return
		}

		slug := strings.TrimSuffix(parts[0], ".webp")
		if !isValidSlug(slug) {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodDelete && len(parts) == 1 {
			uploadHandler.DeleteImage(w, r, slug)
			return
		}

		// /i/{slug}/edit - edit image (GET/POST)
		if len(parts) == 2 && parts[1] == "edit" {
			imageEditHandler.ServeHTTP(w, r, slug)
			return
		}

		// /i/{slug}/restore - restore original (POST)
		if len(parts) == 2 && parts[1] == "restore" {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			imageEditHandler.RestoreOriginal(w, r, slug)
			return
		}

		// /i/{slug} - image view page (HTML)
		if len(parts) == 1 && !strings.HasSuffix(parts[0], ".webp") {
			imageViewHandler.ServeHTTP(w, r, slug)
			return
		}

		// Track access for image files
		go func() {
			_ = db.TouchImageBySlug(slug)
			_ = db.IncrementDownloads(slug)
		}()

		// /i/{slug}/original - original format
		if len(parts) == 2 && parts[1] == "original" {
			uploadHandler.ServeOriginal(w, r, slug)
			return
		}

		sizePart := "original"
		if len(parts) == 2 {
			sizePart = parts[1]
		} else if strings.HasSuffix(parts[0], ".webp") {
			sizePart = "original"
		}
		sizePart = strings.TrimSuffix(sizePart, ".webp")

		originalPath := webpPath(cfg.DataDir, slug, "original")
		thumbPath := webpPath(cfg.DataDir, slug, "thumb")

		switch sizePart {
		case "original", "max":
			if _, err := os.Stat(originalPath); err != nil {
				http.NotFound(w, r)
				return
			}
			serveWebpFile(w, r, originalPath)
			return
		case "thumb":
			if _, err := os.Stat(thumbPath); err != nil {
				http.NotFound(w, r)
				return
			}
			serveWebpFile(w, r, thumbPath)
			return
		default:
			targetWidth, ok := dynamicSizes[sizePart]
			if !ok {
				http.NotFound(w, r)
				return
			}

			origInfo, err := os.Stat(originalPath)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			cacheFile := cachePath(cfg.CacheDir, slug, sizePart)
			if cacheValid(cacheFile, origInfo.ModTime()) {
				logging.Get("cache").Printf("cache hit slug=%s size=%s path=%s", slug, sizePart, cacheFile)
				touch(cacheFile)
				serveWebpFile(w, r, cacheFile)
				return
			}

			resultAny, err, _ := resizeGroup.Do(slug+":"+sizePart, func() (any, error) {
				start := time.Now()
				var wroteCache bool

				origInfo, err := os.Stat(originalPath)
				if err != nil {
					return resizeResult{}, err
				}
				if cacheValid(cacheFile, origInfo.ModTime()) {
					logging.Get("cache").Printf("cache hit slug=%s size=%s path=%s", slug, sizePart, cacheFile)
					return resizeResult{path: cacheFile}, nil
				}

				data, err := os.ReadFile(originalPath)
				if err != nil {
					return resizeResult{}, err
				}
				origWidth, _, err := image.GetSize(data)
				if err != nil {
					return resizeResult{}, err
				}
				if targetWidth >= origWidth {
					return resizeResult{path: originalPath}, nil
				}

				resized, err := image.ResizeToWidth(data, targetWidth)
				if err != nil {
					return resizeResult{}, err
				}
				if err := writeCacheFile(cacheFile, resized); err != nil {
					logging.Get("image").Printf("resize generated slug=%s size=%s target=%d dur_ms=%d cache=miss write=fail", slug, sizePart, targetWidth, time.Since(start).Milliseconds())
					return resizeResult{data: resized}, nil
				}
				wroteCache = true
				logging.Get("image").Printf("resize generated slug=%s size=%s target=%d dur_ms=%d cache=miss write=%t", slug, sizePart, targetWidth, time.Since(start).Milliseconds(), wroteCache)
				return resizeResult{path: cacheFile}, nil
			})
			if err != nil {
				logging.Get("image").Printf("resize error slug=%s size=%s: %v", slug, sizePart, err)
				http.Error(w, "image processing failed", http.StatusInternalServerError)
				return
			}

			result, ok := resultAny.(resizeResult)
			if !ok {
				http.Error(w, "image processing failed", http.StatusInternalServerError)
				return
			}

			if result.path != "" {
				if result.path == cacheFile {
					touch(cacheFile)
				}
				serveWebpFile(w, r, result.path)
				return
			}
			if len(result.data) > 0 {
				serveWebpBytes(w, result.data)
				return
			}

			http.Error(w, "image processing failed", http.StatusInternalServerError)
			return
		}
	})

	log.Printf("Starting server on :%s", cfg.Port)
	handler := requestLogger.Middleware(sessionMiddleware.Middleware(mux))
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}
