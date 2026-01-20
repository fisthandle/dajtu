package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DataDir        string
	MaxFileSizeMB  int
	MaxDiskGB      float64
	CleanupTarget  float64
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8080"),
		DataDir:       getEnv("DATA_DIR", "./data"),
		MaxFileSizeMB: getEnvInt("MAX_FILE_SIZE_MB", 20),
		MaxDiskGB:     getEnvFloat("MAX_DISK_GB", 50.0),
		CleanupTarget: getEnvFloat("CLEANUP_TARGET_GB", 45.0),
	}
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
