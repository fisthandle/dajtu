package config

import (
	"bufio"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func init() {
	loadEnvFile(".env")
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // .env nie istnieje - ok, użyjemy env vars
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Usuń cudzysłowy jeśli są
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Ustaw tylko jeśli nie jest już ustawiona (env vars mają priorytet)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

type Config struct {
	Port               string
	DataDir            string
	LogDir             string
	CacheDir           string
	MaxFileSizeMB      int
	MaxDiskGB          float64
	CleanupTarget      float64
	BaseURL            string
	KeepOriginalFormat bool
	AllowedOrigins     []string // CORS allowed origins
	PublicUpload       bool     // allow upload without login
	AdminNicks         []string // nicks with admin panel access

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
	adminNicks := []string{"KS Amator", "gruby wonsz"}
	if v := os.Getenv("ADMIN_NICKS"); v != "" {
		adminNicks = strings.Split(v, ",")
		for i := range adminNicks {
			adminNicks[i] = strings.TrimSpace(adminNicks[i])
		}
	}

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DataDir:            getEnv("DATA_DIR", "./data"),
		LogDir:             getEnv("LOG_DIR", "./data/logs"),
		CacheDir:           getEnv("CACHE_DIR", "/tmp/dajtu-cache"),
		MaxFileSizeMB:      getEnvInt("MAX_FILE_SIZE_MB", 20),
		MaxDiskGB:          getEnvFloat("MAX_DISK_GB", 50.0),
		CleanupTarget:      getEnvFloat("CLEANUP_TARGET_GB", 45.0),
		BaseURL:            getEnv("BASE_URL", ""),
		KeepOriginalFormat: getEnvBool("KEEP_ORIGINAL_FORMAT", true),
		AllowedOrigins:     parseOrigins(getEnv("ALLOWED_ORIGINS", "")),
		PublicUpload:       getEnvBool("PUBLIC_UPLOAD", true),
		AdminNicks:         adminNicks,

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

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	origins := strings.Split(s, ",")
	result := make([]string, 0, len(origins))
	for _, o := range origins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (c *Config) IsOriginAllowed(origin string) bool {
	if len(c.AllowedOrigins) == 0 {
		return true // no restrictions if not configured
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	originHost := parsed.Hostname() // strips port
	for _, allowed := range c.AllowedOrigins {
		allowedHost := allowed
		if allowedParsed, err := url.Parse(allowed); err == nil && allowedParsed.Host != "" {
			allowedHost = allowedParsed.Hostname()
		}
		if originHost == allowedHost {
			return true
		}
		if strings.HasSuffix(originHost, "."+allowedHost) {
			return true
		}
	}
	return false
}
