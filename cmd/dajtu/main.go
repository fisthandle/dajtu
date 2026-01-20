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
	uploadLimiter := middleware.NewRateLimiter(30, time.Minute)

	mux := http.NewServeMux()

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

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
