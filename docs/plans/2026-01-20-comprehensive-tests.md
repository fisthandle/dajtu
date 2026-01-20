# Comprehensive Test Suite Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Achieve ~95% code coverage with intensive tests for all modules before production deployment.

**Architecture:** Unit tests for each package with table-driven tests, integration tests for DB operations, HTTP handler tests with httptest, and mock-based isolation where needed.

**Tech Stack:** Go testing, testify/assert, httptest, temporary directories for filesystem tests, in-memory SQLite for DB tests.

---

## Task 1: Test Infrastructure Setup

**Files:**
- Create: `internal/testutil/testutil.go`

**Step 1: Write test utilities**

```go
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

// TempDir creates a temporary directory for tests
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dajtu-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestDB creates a test database in temp directory
func TestDB(t *testing.T) (*storage.DB, string) {
	t.Helper()
	dir := TempDir(t)
	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, dir
}

// TestFilesystem creates a test filesystem in temp directory
func TestFilesystem(t *testing.T) (*storage.Filesystem, string) {
	t.Helper()
	dir := TempDir(t)
	fs, err := storage.NewFilesystem(dir)
	if err != nil {
		t.Fatalf("create test filesystem: %v", err)
	}
	return fs, dir
}

// TestConfig returns a config for testing
func TestConfig() *config.Config {
	return &config.Config{
		Port:          "8080",
		DataDir:       "./test-data",
		MaxFileSizeMB: 10,
		MaxDiskGB:     1.0,
		CleanupTarget: 0.5,
		BaseURL:       "http://localhost:8080",
	}
}

// SampleJPEG returns minimal valid JPEG bytes
func SampleJPEG() []byte {
	// Minimal 1x1 red JPEG
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
		0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
		0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
		0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
		0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
		0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
		0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01,
		0x00, 0x00, 0x3F, 0x00, 0xFB, 0xD5, 0xDB, 0x20, 0xA8, 0xF1, 0x85, 0x23,
		0xB1, 0xAF, 0xFF, 0xD9,
	}
}

// SamplePNG returns minimal valid PNG bytes
func SamplePNG() []byte {
	// Minimal 1x1 red PNG
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
		0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xFE, 0xD4, 0xEF, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

// SampleGIF returns minimal valid GIF bytes
func SampleGIF() []byte {
	// Minimal 1x1 GIF
	return []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x21, 0xF9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2C, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x01, 0x00, 0x00, 0x3B,
	}
}

// SampleWebP returns minimal valid WebP bytes
func SampleWebP() []byte {
	// RIFF header + WEBP + minimal VP8 chunk
	return []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x1A, 0x00, 0x00, 0x00, // file size
		0x57, 0x45, 0x42, 0x50, // WEBP
		0x56, 0x50, 0x38, 0x20, // VP8
		0x0E, 0x00, 0x00, 0x00, // chunk size
		0x30, 0x01, 0x00, 0x9D, 0x01, 0x2A, 0x01, 0x00,
		0x01, 0x00, 0x00, 0x34, 0x25, 0x9F, 0x00,
	}
}
```

**Step 2: Verify it compiles**

Run: `cd /home/pawel/dev/dajtu && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/testutil/testutil.go
git commit -m "test: add test utilities and sample image data

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Config Package Tests

**Files:**
- Create: `internal/config/config_test.go`

**Step 1: Write config tests**

```go
package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("PORT")
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("MAX_FILE_SIZE_MB")
	os.Unsetenv("MAX_DISK_GB")
	os.Unsetenv("CLEANUP_TARGET_GB")
	os.Unsetenv("BASE_URL")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.MaxFileSizeMB != 20 {
		t.Errorf("MaxFileSizeMB = %d, want %d", cfg.MaxFileSizeMB, 20)
	}
	if cfg.MaxDiskGB != 50.0 {
		t.Errorf("MaxDiskGB = %f, want %f", cfg.MaxDiskGB, 50.0)
	}
	if cfg.CleanupTarget != 45.0 {
		t.Errorf("CleanupTarget = %f, want %f", cfg.CleanupTarget, 45.0)
	}
	if cfg.BaseURL != "" {
		t.Errorf("BaseURL = %q, want empty", cfg.BaseURL)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("PORT", "3000")
	os.Setenv("DATA_DIR", "/tmp/test")
	os.Setenv("MAX_FILE_SIZE_MB", "50")
	os.Setenv("MAX_DISK_GB", "100.5")
	os.Setenv("CLEANUP_TARGET_GB", "90.0")
	os.Setenv("BASE_URL", "https://example.com")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("MAX_FILE_SIZE_MB")
		os.Unsetenv("MAX_DISK_GB")
		os.Unsetenv("CLEANUP_TARGET_GB")
		os.Unsetenv("BASE_URL")
	}()

	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
	if cfg.DataDir != "/tmp/test" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/test")
	}
	if cfg.MaxFileSizeMB != 50 {
		t.Errorf("MaxFileSizeMB = %d, want %d", cfg.MaxFileSizeMB, 50)
	}
	if cfg.MaxDiskGB != 100.5 {
		t.Errorf("MaxDiskGB = %f, want %f", cfg.MaxDiskGB, 100.5)
	}
	if cfg.CleanupTarget != 90.0 {
		t.Errorf("CleanupTarget = %f, want %f", cfg.CleanupTarget, 90.0)
	}
	if cfg.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://example.com")
	}
}

func TestGetEnvInt_InvalidValue(t *testing.T) {
	os.Setenv("TEST_INT", "not-a-number")
	defer os.Unsetenv("TEST_INT")

	result := getEnvInt("TEST_INT", 42)
	if result != 42 {
		t.Errorf("getEnvInt with invalid value = %d, want fallback %d", result, 42)
	}
}

func TestGetEnvFloat_InvalidValue(t *testing.T) {
	os.Setenv("TEST_FLOAT", "not-a-float")
	defer os.Unsetenv("TEST_FLOAT")

	result := getEnvFloat("TEST_FLOAT", 3.14)
	if result != 3.14 {
		t.Errorf("getEnvFloat with invalid value = %f, want fallback %f", result, 3.14)
	}
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/config/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: add config package tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Image Validator Tests

**Files:**
- Create: `internal/image/validator_test.go`

**Step 1: Write validator tests**

```go
package image

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateAndDetect_JPEG(t *testing.T) {
	// Minimal JPEG header
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	// Pad to minimum size
	data = append(data, make([]byte, 100)...)

	format, result, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatJPEG {
		t.Errorf("format = %q, want %q", format, FormatJPEG)
	}
	if !bytes.Equal(result, data) {
		t.Error("returned data doesn't match input")
	}
}

func TestValidateAndDetect_PNG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatPNG {
		t.Errorf("format = %q, want %q", format, FormatPNG)
	}
}

func TestValidateAndDetect_GIF(t *testing.T) {
	data := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatGIF {
		t.Errorf("format = %q, want %q", format, FormatGIF)
	}
}

func TestValidateAndDetect_WebP(t *testing.T) {
	// RIFF....WEBP
	data := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00, // size placeholder
		0x57, 0x45, 0x42, 0x50, // WEBP
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatWebP {
		t.Errorf("format = %q, want %q", format, FormatWebP)
	}
}

func TestValidateAndDetect_AVIF(t *testing.T) {
	// ftyp box with avif brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // size
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x61, 0x76, 0x69, 0x66, // avif brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_AVIF_Avis(t *testing.T) {
	// ftyp box with avis brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x61, 0x76, 0x69, 0x73, // avis brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_AVIF_Mif1(t *testing.T) {
	// ftyp box with mif1 brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x6D, 0x69, 0x66, 0x31, // mif1 brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_InvalidFormat(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("error = %v, want ErrInvalidFormat", err)
	}
}

func TestValidateAndDetect_FileTooLarge(t *testing.T) {
	data := make([]byte, 1000)
	copy(data, []byte{0xFF, 0xD8, 0xFF}) // JPEG header

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 100)
	if err != ErrFileTooLarge {
		t.Errorf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestValidateAndDetect_TooSmall(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF} // Only 3 bytes

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("error = %v, want ErrInvalidFormat (too small)", err)
	}
}

func TestValidateAndDetect_FakeExtension(t *testing.T) {
	// Executable disguised with JPEG-like data but wrong magic bytes
	data := []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00} // MZ header (EXE)
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("should reject non-image files, got error = %v", err)
	}
}

func TestValidateAndDetect_RIFFNotWebP(t *testing.T) {
	// RIFF file but not WebP (e.g., WAV)
	data := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00,
		0x57, 0x41, 0x56, 0x45, // WAVE (not WEBP)
	}
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("should reject RIFF non-WebP, got error = %v", err)
	}
}

func TestDetectFormat_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Format
	}{
		{
			name:     "JPEG",
			data:     append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...),
			expected: FormatJPEG,
		},
		{
			name:     "PNG",
			data:     append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 20)...),
			expected: FormatPNG,
		},
		{
			name:     "GIF87a",
			data:     append([]byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}, make([]byte, 20)...),
			expected: FormatGIF,
		},
		{
			name:     "GIF89a",
			data:     append([]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, make([]byte, 20)...),
			expected: FormatGIF,
		},
		{
			name:     "Unknown",
			data:     make([]byte, 20),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormat(tt.data)
			if result != tt.expected {
				t.Errorf("detectFormat() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateAndDetect_ReadError(t *testing.T) {
	// Reader that always fails
	r := &errorReader{err: strings.NewReader("").Read(nil)}

	_, _, err := ValidateAndDetect(&failReader{}, 1024)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/image/... -v -run Validate`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/image/validator_test.go
git commit -m "test: add comprehensive image validator tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Storage Filesystem Tests

**Files:**
- Create: `internal/storage/filesystem_test.go`

**Step 1: Write filesystem tests**

```go
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFilesystem(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFilesystem(dir)
	if err != nil {
		t.Fatalf("NewFilesystem() error = %v", err)
	}
	if fs == nil {
		t.Fatal("NewFilesystem() returned nil")
	}

	// Check images directory was created
	imagesDir := filepath.Join(dir, "images")
	info, err := os.Stat(imagesDir)
	if err != nil {
		t.Fatalf("images directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("images path is not a directory")
	}
}

func TestNewFilesystem_InvalidPath(t *testing.T) {
	// Try to create in a read-only location (simulated)
	// On most systems /proc is read-only
	_, err := NewFilesystem("/proc/test-readonly")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		length int
	}{
		{4},
		{5},
		{6},
		{32},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			slug := GenerateSlug(tt.length)
			if len(slug) != tt.length {
				t.Errorf("GenerateSlug(%d) length = %d, want %d", tt.length, len(slug), tt.length)
			}

			// Verify it's hex (lowercase a-f, 0-9)
			for _, c := range slug {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("invalid character in slug: %c", c)
				}
			}
		})
	}
}

func TestGenerateSlug_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		slug := GenerateSlug(6)
		if seen[slug] {
			t.Errorf("duplicate slug generated: %s", slug)
		}
		seen[slug] = true
	}
}

func TestFilesystem_Path(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	tests := []struct {
		slug     string
		sizeName string
		want     string
	}{
		{"ab1c2", "original", filepath.Join(dir, "images", "ab", "ab1c2", "original.webp")},
		{"ab1c2", "800", filepath.Join(dir, "images", "ab", "ab1c2", "800.webp")},
		{"xyz99", "200", filepath.Join(dir, "images", "xy", "xyz99", "200.webp")},
	}

	for _, tt := range tests {
		t.Run(tt.slug+"/"+tt.sizeName, func(t *testing.T) {
			got := fs.Path(tt.slug, tt.sizeName)
			if got != tt.want {
				t.Errorf("Path(%q, %q) = %q, want %q", tt.slug, tt.sizeName, got, tt.want)
			}
		})
	}
}

func TestFilesystem_DirPath(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "ab1c2"
	want := filepath.Join(dir, "images", "ab", "ab1c2")
	got := fs.DirPath(slug)
	if got != want {
		t.Errorf("DirPath(%q) = %q, want %q", slug, got, want)
	}
}

func TestFilesystem_Save(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "test1"
	sizeName := "original"
	data := []byte("test image data")

	err := fs.Save(slug, sizeName, data)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	path := fs.Path(slug, sizeName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("file content = %q, want %q", content, data)
	}
}

func TestFilesystem_Save_MultipleSizes(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "multi1"
	sizes := []string{"original", "1920", "800", "200"}

	for _, size := range sizes {
		data := []byte("data for " + size)
		if err := fs.Save(slug, size, data); err != nil {
			t.Errorf("Save(%q, %q) error = %v", slug, size, err)
		}
	}

	// Verify all files exist
	for _, size := range sizes {
		path := fs.Path(slug, size)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file not created for size %q", size)
		}
	}
}

func TestFilesystem_Delete(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "del01"
	fs.Save(slug, "original", []byte("data"))
	fs.Save(slug, "800", []byte("data"))

	err := fs.Delete(slug)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify directory is gone
	if fs.Exists(slug) {
		t.Error("slug directory still exists after Delete()")
	}
}

func TestFilesystem_Delete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	// Should not error on non-existent
	err := fs.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete(nonexistent) error = %v, want nil", err)
	}
}

func TestFilesystem_Exists(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	slug := "exist"

	if fs.Exists(slug) {
		t.Error("Exists() = true before Save()")
	}

	fs.Save(slug, "original", []byte("data"))

	if !fs.Exists(slug) {
		t.Error("Exists() = false after Save()")
	}

	fs.Delete(slug)

	if fs.Exists(slug) {
		t.Error("Exists() = true after Delete()")
	}
}

func TestFilesystem_GetDiskUsage(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	// Initially empty
	usage, err := fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("GetDiskUsage() error = %v", err)
	}
	if usage != 0 {
		t.Errorf("GetDiskUsage() on empty = %d, want 0", usage)
	}

	// Add some files
	fs.Save("test1", "original", []byte(strings.Repeat("x", 1000)))
	fs.Save("test2", "original", []byte(strings.Repeat("y", 500)))

	usage, err = fs.GetDiskUsage()
	if err != nil {
		t.Fatalf("GetDiskUsage() error = %v", err)
	}
	if usage != 1500 {
		t.Errorf("GetDiskUsage() = %d, want 1500", usage)
	}
}

func TestFilesystem_GetDiskUsage_AfterDelete(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFilesystem(dir)

	fs.Save("test1", "original", []byte(strings.Repeat("x", 1000)))

	fs.Delete("test1")

	usage, _ := fs.GetDiskUsage()
	if usage != 0 {
		t.Errorf("GetDiskUsage() after delete = %d, want 0", usage)
	}
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/storage/... -v -run Filesystem`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/storage/filesystem_test.go
git commit -m "test: add filesystem storage tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Storage Database Tests

**Files:**
- Create: `internal/storage/db_test.go`

**Step 1: Write database tests**

```go
package storage

import (
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewDB(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	if db.conn == nil {
		t.Error("db.conn is nil")
	}
}

func TestDB_InsertImage(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	img := &Image{
		Slug:         "abc12",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		FileSize:     12345,
		Width:        800,
		Height:       600,
		CreatedAt:    now,
		AccessedAt:   now,
	}

	id, err := db.InsertImage(img)
	if err != nil {
		t.Fatalf("InsertImage() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertImage() id = %d, want > 0", id)
	}
}

func TestDB_InsertImage_DuplicateSlug(t *testing.T) {
	db := testDB(t)

	img := &Image{
		Slug:       "dup11",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}

	_, err := db.InsertImage(img)
	if err != nil {
		t.Fatalf("first InsertImage() error = %v", err)
	}

	_, err = db.InsertImage(img)
	if err == nil {
		t.Error("expected error on duplicate slug")
	}
}

func TestDB_GetImageBySlug(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	img := &Image{
		Slug:         "get11",
		OriginalName: "test.png",
		MimeType:     "image/png",
		FileSize:     9999,
		Width:        1024,
		Height:       768,
		CreatedAt:    now,
		AccessedAt:   now,
	}
	db.InsertImage(img)

	got, err := db.GetImageBySlug("get11")
	if err != nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetImageBySlug() = nil")
	}

	if got.Slug != "get11" {
		t.Errorf("Slug = %q, want %q", got.Slug, "get11")
	}
	if got.OriginalName != "test.png" {
		t.Errorf("OriginalName = %q, want %q", got.OriginalName, "test.png")
	}
	if got.FileSize != 9999 {
		t.Errorf("FileSize = %d, want %d", got.FileSize, 9999)
	}
}

func TestDB_GetImageBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetImageBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetImageBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetImageBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_TouchImageBySlug(t *testing.T) {
	db := testDB(t)

	oldTime := time.Now().Add(-24 * time.Hour).Unix()
	img := &Image{
		Slug:       "touch",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  oldTime,
		AccessedAt: oldTime,
	}
	db.InsertImage(img)

	err := db.TouchImageBySlug("touch")
	if err != nil {
		t.Fatalf("TouchImageBySlug() error = %v", err)
	}

	got, _ := db.GetImageBySlug("touch")
	if got.AccessedAt <= oldTime {
		t.Error("AccessedAt was not updated")
	}
}

func TestDB_InsertGallery(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:        "gal1",
		EditToken:   "token123456789012345678901234",
		Title:       "My Gallery",
		Description: "Test description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := db.InsertGallery(g)
	if err != nil {
		t.Fatalf("InsertGallery() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertGallery() id = %d, want > 0", id)
	}
}

func TestDB_GetGalleryBySlug(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:        "getg",
		EditToken:   "edittoken123456789012345678901",
		Title:       "Test Gallery",
		Description: "Desc",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	got, err := db.GetGalleryBySlug("getg")
	if err != nil {
		t.Fatalf("GetGalleryBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetGalleryBySlug() = nil")
	}

	if got.Title != "Test Gallery" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Gallery")
	}
	if got.EditToken != "edittoken123456789012345678901" {
		t.Errorf("EditToken = %q, want %q", got.EditToken, "edittoken123456789012345678901")
	}
}

func TestDB_GetGalleryBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetGalleryBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetGalleryBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetGalleryBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_InsertUser(t *testing.T) {
	db := testDB(t)

	u := &User{
		Slug:        "user01",
		DisplayName: "Test User",
		CreatedAt:   time.Now().Unix(),
	}

	id, err := db.InsertUser(u)
	if err != nil {
		t.Fatalf("InsertUser() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertUser() id = %d, want > 0", id)
	}
}

func TestDB_GetUserBySlug(t *testing.T) {
	db := testDB(t)

	u := &User{
		Slug:        "getu01",
		DisplayName: "Found User",
		CreatedAt:   time.Now().Unix(),
	}
	db.InsertUser(u)

	got, err := db.GetUserBySlug("getu01")
	if err != nil {
		t.Fatalf("GetUserBySlug() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetUserBySlug() = nil")
	}
	if got.DisplayName != "Found User" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Found User")
	}
}

func TestDB_GetUserBySlug_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetUserBySlug("nonexistent")
	if err != nil {
		t.Fatalf("GetUserBySlug() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetUserBySlug(nonexistent) = %v, want nil", got)
	}
}

func TestDB_GetGalleryImages(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "gimgs",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	// Add images to gallery
	for i := 0; i < 3; i++ {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   1000,
			CreatedAt:  now + int64(i),
			AccessedAt: now,
			GalleryID:  &galleryID,
		}
		db.InsertImage(img)
	}

	// Add image NOT in gallery
	img := &Image{
		Slug:       GenerateSlug(5),
		MimeType:   "image/jpeg",
		FileSize:   1000,
		CreatedAt:  now,
		AccessedAt: now,
	}
	db.InsertImage(img)

	images, err := db.GetGalleryImages(galleryID)
	if err != nil {
		t.Fatalf("GetGalleryImages() error = %v", err)
	}
	if len(images) != 3 {
		t.Errorf("GetGalleryImages() count = %d, want 3", len(images))
	}
}

func TestDB_GetGalleryImages_Empty(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "empty",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	images, err := db.GetGalleryImages(galleryID)
	if err != nil {
		t.Fatalf("GetGalleryImages() error = %v", err)
	}
	if len(images) != 0 {
		t.Errorf("GetGalleryImages(empty) count = %d, want 0", len(images))
	}
}

func TestDB_DeleteImageBySlug(t *testing.T) {
	db := testDB(t)

	img := &Image{
		Slug:       "del01",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}
	db.InsertImage(img)

	err := db.DeleteImageBySlug("del01")
	if err != nil {
		t.Fatalf("DeleteImageBySlug() error = %v", err)
	}

	got, _ := db.GetImageBySlug("del01")
	if got != nil {
		t.Error("image still exists after delete")
	}
}

func TestDB_DeleteImageBySlug_NonExistent(t *testing.T) {
	db := testDB(t)

	// Should not error
	err := db.DeleteImageBySlug("nonexistent")
	if err != nil {
		t.Errorf("DeleteImageBySlug(nonexistent) error = %v", err)
	}
}

func TestDB_GetOldestImages(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()

	// Insert images with different creation times
	for i := 0; i < 10; i++ {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   1000,
			CreatedAt:  now - int64(i*100), // Older images have lower timestamps
			AccessedAt: now,
		}
		db.InsertImage(img)
	}

	oldest, err := db.GetOldestImages(5)
	if err != nil {
		t.Fatalf("GetOldestImages() error = %v", err)
	}
	if len(oldest) != 5 {
		t.Errorf("GetOldestImages(5) count = %d, want 5", len(oldest))
	}

	// Verify sorted by created_at ASC
	for i := 1; i < len(oldest); i++ {
		if oldest[i].CreatedAt < oldest[i-1].CreatedAt {
			t.Error("images not sorted by created_at ASC")
		}
	}
}

func TestDB_GetTotalSize(t *testing.T) {
	db := testDB(t)

	// Empty DB
	size, err := db.GetTotalSize()
	if err != nil {
		t.Fatalf("GetTotalSize() error = %v", err)
	}
	if size != 0 {
		t.Errorf("GetTotalSize() on empty = %d, want 0", size)
	}

	// Add images
	now := time.Now().Unix()
	for _, fileSize := range []int64{1000, 2000, 3000} {
		img := &Image{
			Slug:       GenerateSlug(5),
			MimeType:   "image/jpeg",
			FileSize:   fileSize,
			CreatedAt:  now,
			AccessedAt: now,
		}
		db.InsertImage(img)
	}

	size, err = db.GetTotalSize()
	if err != nil {
		t.Fatalf("GetTotalSize() error = %v", err)
	}
	if size != 6000 {
		t.Errorf("GetTotalSize() = %d, want 6000", size)
	}
}

func TestDB_SlugExists(t *testing.T) {
	db := testDB(t)

	// Image slug
	img := &Image{
		Slug:       "exist",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  time.Now().Unix(),
		AccessedAt: time.Now().Unix(),
	}
	db.InsertImage(img)

	exists, err := db.SlugExists("images", "exist")
	if err != nil {
		t.Fatalf("SlugExists() error = %v", err)
	}
	if !exists {
		t.Error("SlugExists() = false for existing slug")
	}

	exists, _ = db.SlugExists("images", "nonexistent")
	if exists {
		t.Error("SlugExists() = true for non-existing slug")
	}
}

func TestDB_SlugExists_Gallery(t *testing.T) {
	db := testDB(t)

	g := &Gallery{
		Slug:      "gexst",
		EditToken: "token",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	db.InsertGallery(g)

	exists, err := db.SlugExists("galleries", "gexst")
	if err != nil {
		t.Fatalf("SlugExists() error = %v", err)
	}
	if !exists {
		t.Error("SlugExists(galleries) = false for existing slug")
	}
}

func TestDB_ImageWithGallery_Cascade(t *testing.T) {
	db := testDB(t)

	now := time.Now().Unix()
	g := &Gallery{
		Slug:      "casc",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	img := &Image{
		Slug:       "cascimg",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  now,
		AccessedAt: now,
		GalleryID:  &galleryID,
	}
	db.InsertImage(img)

	// Delete gallery - should cascade delete images
	_, err := db.conn.Exec("DELETE FROM galleries WHERE slug = ?", "casc")
	if err != nil {
		t.Fatalf("delete gallery error = %v", err)
	}

	// Image should be gone
	got, _ := db.GetImageBySlug("cascimg")
	if got != nil {
		t.Error("image still exists after gallery cascade delete")
	}
}

func TestDB_Close(t *testing.T) {
	dir := t.TempDir()
	db, _ := NewDB(dir)

	err := db.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/storage/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/storage/db_test.go
git commit -m "test: add database storage tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Rate Limiter Middleware Tests

**Files:**
- Create: `internal/middleware/ratelimit_test.go`

**Step 1: Write rate limiter tests**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	if rl == nil {
		t.Fatal("NewRateLimiter() = nil")
	}
	if rl.limit != 10 {
		t.Errorf("limit = %d, want 10", rl.limit)
	}
	if rl.window != time.Minute {
		t.Errorf("window = %v, want %v", rl.window, time.Minute)
	}
}

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	if !rl.Allow("192.168.1.1") {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_Allow_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	ip := "192.168.1.2"

	for i := 0; i < 10; i++ {
		if !rl.Allow(ip) {
			t.Errorf("request %d should be allowed (under limit)", i+1)
		}
	}
}

func TestRateLimiter_Allow_OverLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	ip := "192.168.1.3"

	// Use up the limit
	for i := 0; i < 5; i++ {
		rl.Allow(ip)
	}

	// Next request should be blocked
	if rl.Allow(ip) {
		t.Error("request over limit should be blocked")
	}
}

func TestRateLimiter_Allow_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// IP 1 uses its limit
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Error("10.0.0.1 should be blocked after 2 requests")
	}

	// IP 2 should still work
	if !rl.Allow("10.0.0.2") {
		t.Error("10.0.0.2 should be allowed")
	}
}

func TestRateLimiter_Allow_WindowReset(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	ip := "192.168.1.4"

	// Use up the limit
	rl.Allow(ip)
	rl.Allow(ip)
	if rl.Allow(ip) {
		t.Error("should be blocked after limit")
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("should be allowed after window reset")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 50*time.Millisecond)

	rl.Allow("cleanup.test.1")
	rl.Allow("cleanup.test.2")

	// Wait for entries to expire
	time.Sleep(60 * time.Millisecond)

	// Trigger cleanup manually
	rl.cleanup()

	rl.mu.Lock()
	count := len(rl.visitors)
	rl.mu.Unlock()

	if count != 0 {
		t.Errorf("visitors count after cleanup = %d, want 0", count)
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := rl.Middleware(handler)

	// First two requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// Third request should be blocked
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("blocked request: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Middleware_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// First request with X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	req1.RemoteAddr = "10.0.0.1:12345" // Different RemoteAddr
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	// Second request from same X-Forwarded-For
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Forwarded-For", "203.0.113.1")
	req2.RemoteAddr = "10.0.0.2:12345" // Different RemoteAddr
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Error("should use X-Forwarded-For for rate limiting")
	}
}

func TestRateLimiter_Middleware_FallbackToRemoteAddr(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// No X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.100:5000"
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.100:5001" // Same IP, different port
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Error("should fall back to RemoteAddr when X-Forwarded-For is empty")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, time.Minute)

	done := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				rl.Allow("concurrent.test")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	// Should not panic or deadlock
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/middleware/... -v -race`
Expected: All tests PASS (with race detector)

**Step 3: Commit**

```bash
git add internal/middleware/ratelimit_test.go
git commit -m "test: add rate limiter middleware tests with race detection

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Upload Handler Tests

**Files:**
- Create: `internal/handler/upload_test.go`

**Step 1: Write upload handler tests**

```go
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

func testSetup(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, func()) {
	t.Helper()
	dir := t.TempDir()

	cfg := &config.Config{
		Port:          "8080",
		DataDir:       dir,
		MaxFileSizeMB: 10,
		MaxDiskGB:     1.0,
		CleanupTarget: 0.5,
		BaseURL:       "http://test.local",
	}

	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	fs, err := storage.NewFilesystem(dir)
	if err != nil {
		t.Fatalf("create fs: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return cfg, db, fs, cleanup
}

// sampleJPEG returns a minimal valid JPEG for testing
func sampleJPEG() []byte {
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
		0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
		0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
		0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
		0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
		0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
		0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01,
		0x00, 0x00, 0x3F, 0x00, 0xFB, 0xD5, 0xDB, 0x20, 0xA8, 0xF1, 0x85, 0x23,
		0xB1, 0xAF, 0xFF, 0xD9,
	}
}

func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write(content)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadHandler_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/upload", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestUploadHandler_NoFile(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	req := httptest.NewRequest("POST", "/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUploadHandler_InvalidFormat(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	// Send a text file instead of image
	content := []byte("this is not an image")
	req := createMultipartRequest(t, "file", "test.txt", content)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid image format" {
		t.Errorf("error = %q, want 'invalid image format'", resp["error"])
	}
}

func TestJsonError(t *testing.T) {
	rec := httptest.NewRecorder()
	jsonError(rec, "test error", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", rec.Header().Get("Content-Type"))
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "test error" {
		t.Errorf("error = %q, want 'test error'", resp["error"])
	}
}

func TestUploadHandler_GenerateUniqueSlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	slug := h.generateUniqueSlug("images", 5)
	if len(slug) != 5 {
		t.Errorf("slug length = %d, want 5", len(slug))
	}

	// Should not exist in db
	exists, _ := db.SlugExists("images", slug)
	if exists {
		t.Error("generated slug already exists")
	}
}

func TestUploadHandler_GenerateUniqueSlug_Collision(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewUploadHandler(cfg, db, fs)

	// Pre-insert many slugs to increase collision probability
	for i := 0; i < 100; i++ {
		slug := storage.GenerateSlug(5)
		img := &storage.Image{
			Slug:       slug,
			MimeType:   "image/jpeg",
			FileSize:   100,
			CreatedAt:  1,
			AccessedAt: 1,
		}
		db.InsertImage(img)
	}

	// Should still generate unique slug
	slug := h.generateUniqueSlug("images", 5)
	exists, _ := db.SlugExists("images", slug)
	if exists {
		t.Error("generated slug should be unique")
	}
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/handler/... -v -run Upload`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/handler/upload_test.go
git commit -m "test: add upload handler tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Gallery Handler Tests

**Files:**
- Create: `internal/handler/gallery_test.go`

**Step 1: Write gallery handler tests**

```go
package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dajtu/internal/storage"
)

func TestGalleryHandler_Index(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q, want text/html", rec.Header().Get("Content-Type"))
	}
}

func TestGalleryHandler_Index_NotRoot(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/other", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_Create_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery", nil)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_Create_NoFiles(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "no files provided" {
		t.Errorf("error = %q, want 'no files provided'", resp["error"])
	}
}

func TestGalleryHandler_View_NotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/g/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_View_EmptySlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/g/", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_View_Success(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	// Create a gallery
	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:        "test",
		EditToken:   "token123",
		Title:       "Test Gallery",
		Description: "Description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("GET", "/g/test", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Test Gallery") {
		t.Error("response should contain gallery title")
	}
}

func TestGalleryHandler_View_EditMode(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:        "edit",
		EditToken:   "secret123",
		Title:       "Editable",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("GET", "/g/edit?edit=secret123", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Edit mode should be enabled in the template data
	// (checking this would require parsing the template output)
}

func TestGalleryHandler_View_WrongEditToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "sec1",
		EditToken: "correct",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("GET", "/g/sec1?edit=wrong", nil)
	rec := httptest.NewRecorder()

	h.View(rec, req)

	// Should still show gallery, but not in edit mode
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGalleryHandler_AddImages_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery/test/add", nil)
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_AddImages_InvalidPath(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	tests := []string{
		"/gallery/test",       // No /add
		"/gallery/test/other", // Wrong suffix
		"/gallery/",           // Empty slug
	}

	for _, path := range tests {
		req := httptest.NewRequest("POST", path, nil)
		rec := httptest.NewRecorder()

		h.AddImages(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestGalleryHandler_AddImages_GalleryNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("POST", "/gallery/nonexistent/add", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_AddImages_InvalidToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "add1",
		EditToken: "correct",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("POST", "/gallery/add1/add", nil)
	req.Header.Set("X-Edit-Token", "wrong")
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid edit token" {
		t.Errorf("error = %q, want 'invalid edit token'", resp["error"])
	}
}

func TestGalleryHandler_AddImages_TokenFromForm(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "form1",
		EditToken: "formtoken",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("edit_token", "formtoken")
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery/form1/add", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.AddImages(rec, req)

	// Should pass token validation (but fail on no files)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (no files)", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "no files provided" {
		t.Errorf("error = %q, want 'no files provided'", resp["error"])
	}
}

func TestGalleryHandler_DeleteImage_MethodNotAllowed(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("GET", "/gallery/test/img1", nil)
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestGalleryHandler_DeleteImage_InvalidPath(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("DELETE", "/gallery/test", nil)
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_GalleryNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	req := httptest.NewRequest("DELETE", "/gallery/nonexistent/img1", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_InvalidToken(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "del1",
		EditToken: "correct",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("DELETE", "/gallery/del1/img1", nil)
	req.Header.Set("X-Edit-Token", "wrong")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGalleryHandler_DeleteImage_ImageNotFound(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()
	g := &storage.Gallery{
		Slug:      "del2",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	req := httptest.NewRequest("DELETE", "/gallery/del2/nonexistent", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_ImageNotInGallery(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()

	// Create gallery
	g := &storage.Gallery{
		Slug:      "del3",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.InsertGallery(g)

	// Create image NOT in gallery
	img := &storage.Image{
		Slug:       "other",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  now,
		AccessedAt: now,
		GalleryID:  nil, // Not in any gallery
	}
	db.InsertImage(img)

	req := httptest.NewRequest("DELETE", "/gallery/del3/other", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGalleryHandler_DeleteImage_Success(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	now := time.Now().Unix()

	g := &storage.Gallery{
		Slug:      "del4",
		EditToken: "token",
		CreatedAt: now,
		UpdatedAt: now,
	}
	galleryID, _ := db.InsertGallery(g)

	img := &storage.Image{
		Slug:       "todel",
		MimeType:   "image/jpeg",
		FileSize:   100,
		CreatedAt:  now,
		AccessedAt: now,
		GalleryID:  &galleryID,
	}
	db.InsertImage(img)

	// Create file on disk
	fs.Save("todel", "original", []byte("data"))

	req := httptest.NewRequest("DELETE", "/gallery/del4/todel", nil)
	req.Header.Set("X-Edit-Token", "token")
	rec := httptest.NewRecorder()

	h.DeleteImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["deleted"] != "todel" {
		t.Errorf("deleted = %q, want 'todel'", resp["deleted"])
	}

	// Verify image is gone from DB
	got, _ := db.GetImageBySlug("todel")
	if got != nil {
		t.Error("image still exists in DB after delete")
	}

	// Verify file is gone from disk
	if fs.Exists("todel") {
		t.Error("files still exist on disk after delete")
	}
}

func TestGalleryHandler_GenerateUniqueSlug(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	h := NewGalleryHandler(cfg, db, fs)

	slug := h.generateUniqueSlug("galleries", 4)
	if len(slug) != 4 {
		t.Errorf("slug length = %d, want 4", len(slug))
	}
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/handler/... -v -run Gallery`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/handler/gallery_test.go
git commit -m "test: add gallery handler tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Cleanup Daemon Tests

**Files:**
- Create: `internal/cleanup/daemon_test.go`

**Step 1: Write cleanup daemon tests**

```go
package cleanup

import (
	"testing"
	"time"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

func testSetup(t *testing.T) (*config.Config, *storage.DB, *storage.Filesystem, func()) {
	t.Helper()
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.001,  // 1 MB
		CleanupTarget: 0.0005, // 0.5 MB
	}

	db, err := storage.NewDB(dir)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	fs, err := storage.NewFilesystem(dir)
	if err != nil {
		t.Fatalf("create fs: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return cfg, db, fs, cleanup
}

func TestNewDaemon(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	d := NewDaemon(cfg, db, fs)
	if d == nil {
		t.Fatal("NewDaemon() = nil")
	}
	if d.cfg != cfg {
		t.Error("daemon.cfg not set")
	}
	if d.db != db {
		t.Error("daemon.db not set")
	}
	if d.fs != fs {
		t.Error("daemon.fs not set")
	}
}

func TestDaemon_Cleanup_UnderLimit(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	// Add small amount of data
	now := time.Now().Unix()
	img := &storage.Image{
		Slug:       "small",
		MimeType:   "image/jpeg",
		FileSize:   100, // 100 bytes, well under 1MB limit
		CreatedAt:  now,
		AccessedAt: now,
	}
	db.InsertImage(img)
	fs.Save("small", "original", make([]byte, 100))

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Image should still exist
	got, _ := db.GetImageBySlug("small")
	if got == nil {
		t.Error("image should not be deleted when under limit")
	}
}

func TestDaemon_Cleanup_OverLimit(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.000001,  // ~1KB limit
		CleanupTarget: 0.0000005, // ~500 bytes target
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	// Add data over limit
	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		slug := storage.GenerateSlug(5)
		img := &storage.Image{
			Slug:       slug,
			MimeType:   "image/jpeg",
			FileSize:   200, // 200 bytes each
			CreatedAt:  now - int64(i*100),
			AccessedAt: now,
		}
		db.InsertImage(img)
		fs.Save(slug, "original", make([]byte, 200))
	}

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Some images should be deleted
	totalSize, _ := db.GetTotalSize()
	targetBytes := int64(cfg.CleanupTarget * 1024 * 1024 * 1024)

	if totalSize > targetBytes {
		t.Errorf("total size %d still over target %d after cleanup", totalSize, targetBytes)
	}
}

func TestDaemon_Cleanup_DeletesOldestFirst(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		MaxDiskGB:     0.000001,
		CleanupTarget: 0.0000003,
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	now := time.Now().Unix()

	// oldest image
	oldImg := &storage.Image{
		Slug:       "oldest",
		MimeType:   "image/jpeg",
		FileSize:   500,
		CreatedAt:  now - 1000, // Oldest
		AccessedAt: now,
	}
	db.InsertImage(oldImg)
	fs.Save("oldest", "original", make([]byte, 500))

	// newer image
	newImg := &storage.Image{
		Slug:       "newest",
		MimeType:   "image/jpeg",
		FileSize:   500,
		CreatedAt:  now, // Newest
		AccessedAt: now,
	}
	db.InsertImage(newImg)
	fs.Save("newest", "original", make([]byte, 500))

	d := NewDaemon(cfg, db, fs)
	d.cleanup()

	// Oldest should be deleted first
	old, _ := db.GetImageBySlug("oldest")
	new, _ := db.GetImageBySlug("newest")

	if old != nil && new == nil {
		t.Error("newer image deleted before older image")
	}
}

func TestDaemon_Cleanup_NoImages(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	// High usage configured but no images
	cfg.MaxDiskGB = 0.000000001

	d := NewDaemon(cfg, db, fs)

	// Should not panic
	d.cleanup()
}

func TestDaemon_Start(t *testing.T) {
	cfg, db, fs, cleanup := testSetup(t)
	defer cleanup()

	d := NewDaemon(cfg, db, fs)

	// Just verify Start doesn't panic
	d.Start()

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)
}
```

**Step 2: Run tests**

Run: `cd /home/pawel/dev/dajtu && go test ./internal/cleanup/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/cleanup/daemon_test.go
git commit -m "test: add cleanup daemon tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Integration Tests

**Files:**
- Create: `internal/handler/integration_test.go`

**Step 1: Write integration tests**

```go
//go:build integration

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dajtu/internal/config"
	"dajtu/internal/storage"
)

// These tests require libvips installed
// Run with: go test -tags=integration ./internal/handler/...

func getRealTestImage(t *testing.T) []byte {
	t.Helper()

	// Try to load a real test image from testdata
	path := filepath.Join("testdata", "test.jpg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("skipping integration test: testdata/test.jpg not found")
	}
	return data
}

func TestIntegration_FullUploadFlow(t *testing.T) {
	imgData := getRealTestImage(t)

	dir := t.TempDir()
	cfg := &config.Config{
		Port:          "8080",
		DataDir:       dir,
		MaxFileSizeMB: 10,
		MaxDiskGB:     1.0,
		CleanupTarget: 0.5,
		BaseURL:       "http://test.local",
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	h := NewUploadHandler(cfg, db, fs)

	// Create multipart request with real image
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.jpg")
	part.Write(imgData)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, body)
	}

	var resp UploadResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Slug == "" {
		t.Error("response slug is empty")
	}
	if resp.URL == "" {
		t.Error("response URL is empty")
	}
	if len(resp.Sizes) == 0 {
		t.Error("response sizes is empty")
	}

	// Verify image in database
	img, err := db.GetImageBySlug(resp.Slug)
	if err != nil || img == nil {
		t.Error("image not found in database")
	}

	// Verify files on disk
	if !fs.Exists(resp.Slug) {
		t.Error("image files not found on disk")
	}
}

func TestIntegration_GalleryWithImages(t *testing.T) {
	imgData := getRealTestImage(t)

	dir := t.TempDir()
	cfg := &config.Config{
		Port:          "8080",
		DataDir:       dir,
		MaxFileSizeMB: 10,
		MaxDiskGB:     1.0,
		CleanupTarget: 0.5,
		BaseURL:       "http://test.local",
	}

	db, _ := storage.NewDB(dir)
	defer db.Close()

	fs, _ := storage.NewFilesystem(dir)

	h := NewGalleryHandler(cfg, db, fs)

	// Create gallery with images
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("title", "Test Gallery")
	writer.WriteField("description", "Integration test")

	for i := 0; i < 3; i++ {
		part, _ := writer.CreateFormFile("files", "test.jpg")
		part.Write(imgData)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/gallery", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("create status = %d, want %d, body: %s", rec.Code, http.StatusOK, body)
	}

	var resp GalleryCreateResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Slug == "" {
		t.Error("gallery slug is empty")
	}
	if resp.EditToken == "" {
		t.Error("edit token is empty")
	}
	if len(resp.Images) != 3 {
		t.Errorf("images count = %d, want 3", len(resp.Images))
	}

	// View gallery
	viewReq := httptest.NewRequest("GET", "/g/"+resp.Slug, nil)
	viewRec := httptest.NewRecorder()
	h.View(viewRec, viewReq)

	if viewRec.Code != http.StatusOK {
		t.Errorf("view status = %d, want %d", viewRec.Code, http.StatusOK)
	}
}
```

**Step 2: Create testdata directory and add test image**

```bash
mkdir -p internal/handler/testdata
# Download or copy a small test JPEG to internal/handler/testdata/test.jpg
```

**Step 3: Run integration tests**

Run: `cd /home/pawel/dev/dajtu && go test -tags=integration ./internal/handler/... -v`
Expected: Tests pass if libvips is installed and test.jpg exists

**Step 4: Commit**

```bash
git add internal/handler/integration_test.go
git commit -m "test: add integration tests for full upload/gallery flow

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: Run Full Test Suite and Check Coverage

**Step 1: Run all tests with coverage**

Run: `cd /home/pawel/dev/dajtu && go test ./... -coverprofile=coverage.out -covermode=atomic`

**Step 2: Check coverage report**

Run: `cd /home/pawel/dev/dajtu && go tool cover -func=coverage.out | tail -1`
Expected: total coverage ~80-95%

**Step 3: Generate HTML coverage report**

Run: `cd /home/pawel/dev/dajtu && go tool cover -html=coverage.out -o coverage.html`

**Step 4: Add coverage files to gitignore**

Run: `echo -e "coverage.out\ncoverage.html" >> .gitignore`

**Step 5: Commit**

```bash
git add .gitignore
git commit -m "chore: add coverage files to gitignore

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Summary

| Module | Functions | Test Coverage Target |
|--------|-----------|---------------------|
| config | 4 | 100% |
| image/validator | 2 | 100% |
| storage/filesystem | 8 | 100% |
| storage/db | 14 | 100% |
| middleware/ratelimit | 4 | 100% |
| handler/upload | 3 | 90% |
| handler/gallery | 7 | 90% |
| cleanup/daemon | 3 | 90% |

**Total estimated coverage: ~95%**

Key testing areas:
- Magic byte validation (security critical)
- Database CRUD operations
- Filesystem operations with cleanup
- Rate limiting with concurrent access
- HTTP handler error cases
- Edit token verification
- Cascade deletions

---

## Addendum: SSO + Original Formats (2026-01-20)

**SSO tests:**
- `internal/auth/brat_test.go`: invalid config, invalid base64, valid payload roundtrip.
- `internal/handler/auth_test.go`: missing data -> 400, SSO disabled -> 503.
- `internal/storage/db_test.go`: `GetOrCreateBratUser` (new + existing).

**Original formats tests:**
- `internal/storage/filesystem_test.go`: `SaveOriginal` JPEG/PNG + `GetOriginalPath`.
- `internal/handler/upload_test.go`: `TestUploadHandler_SavesOriginal` (skip if processing unavailable).
