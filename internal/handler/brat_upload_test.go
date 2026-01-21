package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"dajtu/internal/auth"
	"dajtu/internal/testutil"
)

func TestBratUpload_InvalidToken(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	content := testutil.SampleJPEG()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("image", "test.jpg")
	part.Write(content)
	writer.Close()

	req := httptest.NewRequest("POST", "/brtup/invalid-token/123/nope", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid or expired token" {
		t.Errorf("error = %q, want 'invalid or expired token'", resp["error"])
	}
}

func TestBratUpload_MissingImage(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("other", "value")
	writer.Close()

	req := httptest.NewRequest("POST", "/brtup/token/123/nope", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (token validation happens before file check)", rec.Code, http.StatusUnauthorized)
	}
}

func TestBratUpload_InvalidPath(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	tests := []string{
		"/brtup/token",
		"/brtup/token/123",
		"/brtup/",
		"/brtup/a/b/c/d",
	}

	for _, path := range tests {
		req := httptest.NewRequest("POST", path, nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("path %q: status = %d, want %d", path, rec.Code, http.StatusBadRequest)
		}

		var resp map[string]string
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["error"] != "invalid path format" {
			t.Errorf("path %q: error = %q, want 'invalid path format'", path, resp["error"])
		}
	}
}

func TestBratUpload_CORSHeaders(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	origins := []string{
		"https://braterstwo.pl",
		"https://forum.braterstwo.com",
		"http://localhost:3000",
	}

	for _, origin := range origins {
		req := httptest.NewRequest("POST", "/brtup/token/123/nope", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("origin %q: CORS header = %q, want %q",
				origin,
				rec.Header().Get("Access-Control-Allow-Origin"),
				origin)
		}

		if rec.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
			t.Errorf("origin %q: methods header = %q, want 'POST, OPTIONS'",
				origin,
				rec.Header().Get("Access-Control-Allow-Methods"))
		}

		if rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
			t.Errorf("origin %q: headers = %q, want 'Content-Type'",
				origin,
				rec.Header().Get("Access-Control-Allow-Headers"))
		}
	}
}

func TestBratUpload_CORSHeaders_InvalidOrigin(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	req := httptest.NewRequest("POST", "/brtup/token/123/nope", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("evil origin should not get CORS headers, got: %q",
			rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBratUpload_OptionsRequest(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	req := httptest.NewRequest("OPTIONS", "/brtup/token/123/nope", nil)
	req.Header.Set("Origin", "https://braterstwo.pl")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://braterstwo.pl" {
		t.Errorf("CORS header = %q, want 'https://braterstwo.pl'",
			rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBratUpload_DecoderNil(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	h := NewBratUploadHandler(cfg, db, fs, nil)

	req := httptest.NewRequest("POST", "/brtup/token/123/nope", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "SSO not configured" {
		t.Errorf("error = %q, want 'SSO not configured'", resp["error"])
	}
}

func TestBratUpload_TitleDecoding(t *testing.T) {
	tests := []struct {
		titleBase64 string
		want        string
	}{
		{"nope", "Nowe wątki"},
		{base64.URLEncoding.EncodeToString([]byte("Test Title")), "Test Title"},
		{base64.URLEncoding.EncodeToString([]byte("Wątek po polsku")), "Wątek po polsku"},
	}

	for _, tt := range tests {
		titleBytes, _ := base64.URLEncoding.DecodeString(tt.titleBase64)
		if tt.titleBase64 == "nope" {
			if tt.want != "Nowe wątki" {
				t.Errorf("nope should decode to 'Nowe wątki', got %q", tt.want)
			}
		} else {
			if string(titleBytes) != tt.want {
				t.Errorf("title = %q, want %q", string(titleBytes), tt.want)
			}
		}
	}
}

func TestBratUpload_MethodNotAllowed(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	req := httptest.NewRequest("GET", "/brtup/token/123/nope", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestBratUpload_InvalidImageFormat(t *testing.T) {
	cfg := testutil.TestConfig(t)
	db, _ := testutil.TestDB(t)
	fs, _ := testutil.TestFilesystem(t)

	decoder, _ := auth.NewBratDecoder(auth.BratConfig{
		HashSecret:        "test",
		EncryptionKey:     "testkey",
		EncryptionIV:      "1234567890123456",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    3600,
		HashLength:        16,
		HashBytes:         8,
		MaxPseudonimBytes: 64,
	})

	h := NewBratUploadHandler(cfg, db, fs, decoder)

	content := []byte("not an image")
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("image", "test.txt")
	part.Write(content)
	writer.Close()

	req := httptest.NewRequest("POST", "/brtup/invalid/123/nope", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (auth happens before image validation)", rec.Code, http.StatusUnauthorized)
	}
}

func TestBratUpload_GenerateFunctions(t *testing.T) {
	db, _ := testutil.TestDB(t)

	slug := generateUniqueSlug(db, "images", 5)
	if len(slug) != 5 {
		t.Errorf("slug length = %d, want 5", len(slug))
	}

	exists, _ := db.SlugExists("images", slug)
	if exists {
		t.Error("generated slug should not exist")
	}

	token := generateEditToken()
	if len(token) != 32 {
		t.Errorf("token length = %d, want 32", len(token))
	}
}
