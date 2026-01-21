package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dajtu/internal/config"
)

// jsonError wysyła błąd JSON
func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// getBaseURL zwraca base URL z konfiguracji lub z requestu
func getBaseURL(cfg *config.Config, r *http.Request) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// buildImageURL buduje URL do obrazka
func buildImageURL(baseURL, slug, size string) string {
	if size == "" || size == "original" {
		return fmt.Sprintf("%s/i/%s.webp", baseURL, slug)
	}
	return fmt.Sprintf("%s/i/%s/%s.webp", baseURL, slug, size)
}

// generateEditToken generuje 32-znakowy token
func generateEditToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:32], nil
	}
	return hex.EncodeToString(b), nil
}
