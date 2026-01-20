package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"dajtu/internal/auth"
	"dajtu/internal/config"
	"dajtu/internal/storage"
)

type AuthHandler struct {
	cfg     *config.Config
	db      *storage.DB
	decoder *auth.BratDecoder
}

func NewAuthHandler(cfg *config.Config, db *storage.DB) (*AuthHandler, error) {
	decoder, err := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        cfg.BratHashSecret,
		EncryptionKey:     cfg.BratEncryptionKey,
		EncryptionIV:      cfg.BratEncryptionIV,
		Cipher:            cfg.BratCipher,
		MaxSkewSeconds:    cfg.BratMaxSkewSeconds,
		HashLength:        cfg.BratHashLength,
		HashBytes:         cfg.BratHashBytes,
		MaxPseudonimBytes: cfg.BratMaxPseudonimBytes,
	})
	if err != nil {
		return nil, err
	}

	return &AuthHandler{cfg: cfg, db: db, decoder: decoder}, nil
}

func (h *AuthHandler) HandleBratSSO(w http.ResponseWriter, r *http.Request) {
	if h.decoder == nil {
		http.Error(w, "SSO not configured", http.StatusServiceUnavailable)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/auth/brat/")
	data := strings.TrimSuffix(path, "/")
	if data == "" {
		data = r.URL.Query().Get("data")
	}
	if data == "" {
		http.Error(w, "missing data parameter", http.StatusBadRequest)
		return
	}

	user, err := h.decoder.Decode(data)
	if err != nil {
		log.Printf("SSO decode error: %v", err)
		http.Error(w, "invalid SSO payload", http.StatusUnauthorized)
		return
	}

	dbUser, err := h.db.GetOrCreateBratUser(user.Pseudonim, user.Punktacja)
	if err != nil {
		log.Printf("SSO user error: %v", err)
		http.Error(w, "user creation failed", http.StatusInternalServerError)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"user_id":   dbUser.ID,
			"slug":      dbUser.Slug,
			"name":      dbUser.DisplayName,
			"punktacja": user.Punktacja,
		})
		return
	}

	http.Redirect(w, r, "/u/"+dbUser.Slug, http.StatusFound)
}
