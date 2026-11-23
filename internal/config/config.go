package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from environment variables (and optional .env file).
type Config struct {
	Host              string
	Port              string
	SiteURL           string
	DataDir           string
	AdminEmail        string
	AdminPasswordHash string
	SessionSecret     []byte
	TrustProxy        bool
	MaxUploadBytes    int64
	GitHubAPIURL      string
}

// Load reads optional .env file from the working directory (without overriding existing env vars)
// and returns a validated Config with sensible defaults.
func Load() (*Config, error) {
	loadDotEnv(".env")

	host := getEnv("HOST", "127.0.0.1")
	port := getEnv("PORT", "8080")
	siteURL := strings.TrimRight(getEnv("SITE_URL", "https://neddyap.tech"), "/")
	if siteURL == "" {
		siteURL = "https://neddyap.tech"
	}
	dataDir := getEnv("DATA_DIR", "./data")
	adminEmail := strings.TrimSpace(getEnv("ADMIN_EMAIL", "neddy.prasetio@gmail.com"))
	adminHash := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH"))

	secretStr := os.Getenv("SESSION_SECRET")
	var secret []byte
	if len(secretStr) >= 32 {
		secret = []byte(secretStr)
	} else {
		if secretStr != "" {
			slog.Warn("SESSION_SECRET is shorter than 32 bytes; generating a temporary secret")
		} else {
			slog.Warn("SESSION_SECRET not set; generating ephemeral secret (sessions reset on restart)")
		}
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		secret = []byte(hex.EncodeToString(buf))
	}

	trustProxy := true
	if v := strings.TrimSpace(os.Getenv("TRUST_PROXY")); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			trustProxy = parsed
		}
	}

	maxUploadMB := int64(5)
	if v := strings.TrimSpace(os.Getenv("MAX_UPLOAD_MB")); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 && parsed <= 50 {
			maxUploadMB = parsed
		}
	}

	githubAPIURL := strings.TrimRight(getEnv("GITHUB_API_URL", "https://github-contributions-api.jogruber.de/v4"), "/")

	return &Config{
		Host:              host,
		Port:              port,
		SiteURL:           siteURL,
		DataDir:           dataDir,
		AdminEmail:        adminEmail,
		AdminPasswordHash: adminHash,
		SessionSecret:     secret,
		TrustProxy:        trustProxy,
		MaxUploadBytes:    maxUploadMB * 1024 * 1024,
		GitHubAPIURL:      githubAPIURL,
	}, nil
}

// Address returns host:port listen address.
func (c *Config) Address() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
