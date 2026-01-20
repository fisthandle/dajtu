package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dajtu/internal/config"
	"dajtu/internal/testutil"
)

func TestAuthHandler_MissingData(t *testing.T) {
	cfg := testutil.TestConfig(t)
	cfg.BratHashSecret = "70bea0d8db1666ebb2c0aef1a820be13d8274dca73843935336f2dbb0e2f4f7e"
	cfg.BratEncryptionKey = "8d29fa415eda466a6e54d2843b5d219295de5fe9c5a52330c88a9f9b7107b0e4"
	cfg.BratEncryptionIV = "eOnipVEZxUk52ezJ"
	cfg.BratCipher = "AES-256-CBC"
	cfg.BratMaxSkewSeconds = 600
	cfg.BratHashLength = 10
	cfg.BratHashBytes = 5
	cfg.BratMaxPseudonimBytes = 255

	db, _ := testutil.TestDB(t)

	h, err := NewAuthHandler(cfg, db)
	if err != nil {
		t.Fatalf("NewAuthHandler error: %v", err)
	}

	req := httptest.NewRequest("GET", "/auth/brat/", nil)
	w := httptest.NewRecorder()

	h.HandleBratSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_SSODisabled(t *testing.T) {
	cfg := &config.Config{}
	db, _ := testutil.TestDB(t)

	h, err := NewAuthHandler(cfg, db)
	if err != nil {
		t.Fatalf("NewAuthHandler error: %v", err)
	}

	req := httptest.NewRequest("GET", "/auth/brat/somedata", nil)
	w := httptest.NewRecorder()

	h.HandleBratSSO(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}
