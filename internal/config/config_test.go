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
