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
