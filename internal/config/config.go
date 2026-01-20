package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DataDir            string
	MaxFileSizeMB      int
	MaxDiskGB          float64
	CleanupTarget      float64
	BaseURL            string
	KeepOriginalFormat bool

	// SSO Braterstwo
	BratHashSecret        string
	BratEncryptionKey     string
	BratEncryptionIV      string
	BratCipher            string
	BratMaxSkewSeconds    int
	BratHashLength        int
	BratHashBytes         int
	BratMaxPseudonimBytes int
}

func Load() *Config {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DataDir:            getEnv("DATA_DIR", "./data"),
		MaxFileSizeMB:      getEnvInt("MAX_FILE_SIZE_MB", 20),
		MaxDiskGB:          getEnvFloat("MAX_DISK_GB", 50.0),
		CleanupTarget:      getEnvFloat("CLEANUP_TARGET_GB", 45.0),
		BaseURL:            getEnv("BASE_URL", ""),
		KeepOriginalFormat: getEnvBool("KEEP_ORIGINAL_FORMAT", true),

		BratHashSecret:        getEnv("BRAT_HASH_SECRET", ""),
		BratEncryptionKey:     getEnv("BRAT_ENCRYPTION_KEY", ""),
		BratEncryptionIV:      getEnv("BRAT_ENCRYPTION_IV", ""),
		BratCipher:            getEnv("BRAT_CIPHER", "AES-256-CBC"),
		BratMaxSkewSeconds:    getEnvInt("BRAT_MAX_SKEW_SECONDS", 600),
		BratHashLength:        getEnvInt("BRAT_HASH_LENGTH", 10),
		BratHashBytes:         getEnvInt("BRAT_HASH_BYTES", 5),
		BratMaxPseudonimBytes: getEnvInt("BRAT_MAX_PSEUDONIM_BYTES", 255),
	}

	// Validate: CleanupTarget must be less than MaxDiskGB
	if cfg.CleanupTarget >= cfg.MaxDiskGB {
		cfg.CleanupTarget = cfg.MaxDiskGB * 0.9
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1"
	}
	return fallback
}
