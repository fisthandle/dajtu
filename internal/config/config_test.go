package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("PORT")
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("LOG_DIR")
	os.Unsetenv("CACHE_DIR")
	os.Unsetenv("MAX_FILE_SIZE_MB")
	os.Unsetenv("MAX_DISK_GB")
	os.Unsetenv("CLEANUP_TARGET_GB")
	os.Unsetenv("BASE_URL")
	os.Unsetenv("KEEP_ORIGINAL_FORMAT")
	os.Unsetenv("ALLOWED_ORIGINS")
	os.Unsetenv("PUBLIC_UPLOAD")
	os.Unsetenv("BRAT_HASH_SECRET")
	os.Unsetenv("BRAT_ENCRYPTION_KEY")
	os.Unsetenv("BRAT_ENCRYPTION_IV")
	os.Unsetenv("BRAT_CIPHER")
	os.Unsetenv("BRAT_MAX_SKEW_SECONDS")
	os.Unsetenv("BRAT_HASH_LENGTH")
	os.Unsetenv("BRAT_HASH_BYTES")
	os.Unsetenv("BRAT_MAX_PSEUDONIM_BYTES")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.LogDir != "./data/logs" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, "./data/logs")
	}
	if cfg.CacheDir != "/tmp/dajtu-cache" {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/tmp/dajtu-cache")
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
	if !cfg.KeepOriginalFormat {
		t.Errorf("KeepOriginalFormat = %v, want true", cfg.KeepOriginalFormat)
	}
	if cfg.AllowedOrigins != nil {
		t.Errorf("AllowedOrigins = %v, want nil", cfg.AllowedOrigins)
	}
	if !cfg.PublicUpload {
		t.Errorf("PublicUpload = %v, want true", cfg.PublicUpload)
	}
	if cfg.BratCipher != "AES-256-CBC" {
		t.Errorf("BratCipher = %q, want %q", cfg.BratCipher, "AES-256-CBC")
	}
	if cfg.BratMaxSkewSeconds != 600 {
		t.Errorf("BratMaxSkewSeconds = %d, want 600", cfg.BratMaxSkewSeconds)
	}
	if cfg.BratHashLength != 10 {
		t.Errorf("BratHashLength = %d, want 10", cfg.BratHashLength)
	}
	if cfg.BratHashBytes != 5 {
		t.Errorf("BratHashBytes = %d, want 5", cfg.BratHashBytes)
	}
	if cfg.BratMaxPseudonimBytes != 255 {
		t.Errorf("BratMaxPseudonimBytes = %d, want 255", cfg.BratMaxPseudonimBytes)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("PORT", "3000")
	os.Setenv("DATA_DIR", "/tmp/test")
	os.Setenv("LOG_DIR", "/tmp/logs")
	os.Setenv("CACHE_DIR", "/tmp/cache")
	os.Setenv("MAX_FILE_SIZE_MB", "50")
	os.Setenv("MAX_DISK_GB", "100.5")
	os.Setenv("CLEANUP_TARGET_GB", "90.0")
	os.Setenv("BASE_URL", "https://example.com")
	os.Setenv("KEEP_ORIGINAL_FORMAT", "0")
	os.Setenv("BRAT_HASH_SECRET", "test_hash_secret")
	os.Setenv("BRAT_ENCRYPTION_KEY", "test_encryption_key")
	os.Setenv("BRAT_ENCRYPTION_IV", "1234567890123456")
	os.Setenv("BRAT_CIPHER", "AES-256-CBC")
	os.Setenv("BRAT_MAX_SKEW_SECONDS", "900")
	os.Setenv("BRAT_HASH_LENGTH", "12")
	os.Setenv("BRAT_HASH_BYTES", "6")
	os.Setenv("BRAT_MAX_PSEUDONIM_BYTES", "128")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("LOG_DIR")
		os.Unsetenv("CACHE_DIR")
		os.Unsetenv("MAX_FILE_SIZE_MB")
		os.Unsetenv("MAX_DISK_GB")
		os.Unsetenv("CLEANUP_TARGET_GB")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("KEEP_ORIGINAL_FORMAT")
		os.Unsetenv("BRAT_HASH_SECRET")
		os.Unsetenv("BRAT_ENCRYPTION_KEY")
		os.Unsetenv("BRAT_ENCRYPTION_IV")
		os.Unsetenv("BRAT_CIPHER")
		os.Unsetenv("BRAT_MAX_SKEW_SECONDS")
		os.Unsetenv("BRAT_HASH_LENGTH")
		os.Unsetenv("BRAT_HASH_BYTES")
		os.Unsetenv("BRAT_MAX_PSEUDONIM_BYTES")
	}()

	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
	if cfg.DataDir != "/tmp/test" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/test")
	}
	if cfg.LogDir != "/tmp/logs" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, "/tmp/logs")
	}
	if cfg.CacheDir != "/tmp/cache" {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/tmp/cache")
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
	if cfg.KeepOriginalFormat {
		t.Errorf("KeepOriginalFormat = %v, want false", cfg.KeepOriginalFormat)
	}
	if cfg.BratHashSecret != "test_hash_secret" {
		t.Errorf("BratHashSecret = %q, want %q", cfg.BratHashSecret, "test_hash_secret")
	}
	if cfg.BratEncryptionKey != "test_encryption_key" {
		t.Errorf("BratEncryptionKey = %q, want %q", cfg.BratEncryptionKey, "test_encryption_key")
	}
	if cfg.BratEncryptionIV != "1234567890123456" {
		t.Errorf("BratEncryptionIV = %q, want %q", cfg.BratEncryptionIV, "1234567890123456")
	}
	if cfg.BratMaxSkewSeconds != 900 {
		t.Errorf("BratMaxSkewSeconds = %d, want 900", cfg.BratMaxSkewSeconds)
	}
	if cfg.BratHashLength != 12 {
		t.Errorf("BratHashLength = %d, want 12", cfg.BratHashLength)
	}
	if cfg.BratHashBytes != 6 {
		t.Errorf("BratHashBytes = %d, want 6", cfg.BratHashBytes)
	}
	if cfg.BratMaxPseudonimBytes != 128 {
		t.Errorf("BratMaxPseudonimBytes = %d, want 128", cfg.BratMaxPseudonimBytes)
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

func TestLoad_CleanupTargetValidation(t *testing.T) {
	// Set invalid values: CleanupTarget >= MaxDiskGB
	os.Setenv("MAX_DISK_GB", "50")
	os.Setenv("CLEANUP_TARGET_GB", "60") // Invalid: greater than max
	defer func() {
		os.Unsetenv("MAX_DISK_GB")
		os.Unsetenv("CLEANUP_TARGET_GB")
	}()

	cfg := Load()

	// Should auto-correct to 90% of MaxDiskGB
	expected := 50.0 * 0.9
	if cfg.CleanupTarget != expected {
		t.Errorf("CleanupTarget = %v, want %v (90%% of MaxDiskGB)", cfg.CleanupTarget, expected)
	}
}

func TestLoad_CleanupTargetValidation_Equal(t *testing.T) {
	// Set CleanupTarget equal to MaxDiskGB (also invalid)
	os.Setenv("MAX_DISK_GB", "50")
	os.Setenv("CLEANUP_TARGET_GB", "50")
	defer func() {
		os.Unsetenv("MAX_DISK_GB")
		os.Unsetenv("CLEANUP_TARGET_GB")
	}()

	cfg := Load()

	expected := 50.0 * 0.9
	if cfg.CleanupTarget != expected {
		t.Errorf("CleanupTarget = %v, want %v (90%% of MaxDiskGB)", cfg.CleanupTarget, expected)
	}
}

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"example.com", []string{"example.com"}},
		{"a.com,b.com", []string{"a.com", "b.com"}},
		{"a.com, b.com, c.com", []string{"a.com", "b.com", "c.com"}},
		{" a.com , b.com ", []string{"a.com", "b.com"}},
	}

	for _, tt := range tests {
		got := parseOrigins(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseOrigins(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseOrigins(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		allowed []string
		origin  string
		want    bool
	}{
		{nil, "https://any.com", true},                                   // no restrictions
		{[]string{}, "https://any.com", true},                            // empty = no restrictions
		{[]string{"example.com"}, "https://example.com", true},           // exact match
		{[]string{"example.com"}, "https://sub.example.com", true},       // subdomain
		{[]string{"example.com"}, "https://evil.com", false},             // not allowed
		{[]string{"a.com", "b.com"}, "https://a.com", true},              // multiple allowed
		{[]string{"a.com", "b.com"}, "https://c.com", false},             // not in list
		{[]string{"localhost"}, "http://localhost:3000", true},           // localhost with port
		{[]string{"braterstwo.eu"}, "https://forum.braterstwo.eu", true}, // subdomain match
	}

	for _, tt := range tests {
		cfg := &Config{AllowedOrigins: tt.allowed}
		got := cfg.IsOriginAllowed(tt.origin)
		if got != tt.want {
			t.Errorf("IsOriginAllowed(%v, %q) = %v, want %v", tt.allowed, tt.origin, got, tt.want)
		}
	}
}

func TestLoad_AllowedOrigins(t *testing.T) {
	os.Setenv("ALLOWED_ORIGINS", "braterstwo.eu,localhost")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg := Load()

	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins length = %d, want 2", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "braterstwo.eu" {
		t.Errorf("AllowedOrigins[0] = %q, want %q", cfg.AllowedOrigins[0], "braterstwo.eu")
	}
}

func TestLoad_PublicUpload(t *testing.T) {
	os.Setenv("PUBLIC_UPLOAD", "false")
	defer os.Unsetenv("PUBLIC_UPLOAD")

	cfg := Load()

	if cfg.PublicUpload {
		t.Error("PublicUpload = true, want false")
	}
}
