package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/cleanup"
	"dajtu/internal/config"
	"dajtu/internal/handler"
	"dajtu/internal/middleware"
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

	cleanupDaemon := cleanup.NewDaemon(cfg, db, fs)
	cleanupDaemon.Start()

	uploadHandler := handler.NewUploadHandler(cfg, db, fs)
	galleryHandler := handler.NewGalleryHandler(cfg, db, fs)
	authHandler, err := handler.NewAuthHandler(cfg, db)
	if err != nil {
		log.Fatalf("Failed to init SSO: %v", err)
	}
	userHandler := handler.NewUserHandler(cfg, db)
	uploadLimiter := middleware.NewRateLimiter(30, time.Minute)
	sessionMiddleware := middleware.NewSessionMiddleware(db)

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
		if len(slug) < 2 {
			http.NotFound(w, r)
			return
		}

		if len(parts) == 2 && parts[1] == "original" {
			uploadHandler.ServeOriginal(w, r, slug)
			return
		}

		prefix := slug[:2]
		size := "original.webp"

		if len(parts) == 2 {
			size = parts[1]
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "image/webp")

		filePath := cfg.DataDir + "/images/" + prefix + "/" + slug + "/" + size
		http.ServeFile(w, r, filePath)
	})

	log.Printf("Starting server on :%s", cfg.Port)
	handler := sessionMiddleware.Middleware(mux)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}
