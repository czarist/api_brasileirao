// Package config loads the process configuration from the environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/catalog"
)

// Config is the validated runtime configuration.
type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	CORSOrigins       []string
	RateLimitRPS      float64
	RateLimitBurst    int
	TrustProxy        bool
	LogLevel          string
	SourceBaseURL     string
	SourceTimeout     time.Duration
	SourceMinInterval time.Duration
	SourceMaxBody     int64
	SourceRetries     int
	SyncInterval      time.Duration
	SyncTimeout       time.Duration
	SyncCompetition   string
	SyncYear          int
	SyncAllYears      bool
	StaleAfter        time.Duration
	MinMatches        int
	MinStandings      int
	MaxStandings      int
	DBMaxConns        int32
}

// LoadDotEnv reads path into the environment without overriding variables that
// are already set. A missing file is ignored.
func LoadDotEnv(path string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}

// Load reads and validates the environment.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   15 * time.Second,
		CORSOrigins:       splitCSV(getenv("CORS_ALLOWED_ORIGINS", "*")),
		RateLimitRPS:      10,
		RateLimitBurst:    20,
		LogLevel:          strings.ToLower(getenv("LOG_LEVEL", "info")),
		SourceBaseURL:     getenv("SOURCE_BASE_URL", "https://www.cbf.com.br"),
		SourceTimeout:     30 * time.Second,
		SourceMinInterval: time.Second,
		SourceMaxBody:     8 << 20,
		SourceRetries:     2,
		SyncInterval:      time.Hour,
		SyncTimeout:       2 * time.Minute,
		SyncCompetition:   getenv("SYNC_COMPETITION", "brasileirao-serie-a"),
		StaleAfter:        6 * time.Hour,
		MinMatches:        100,
		MinStandings:      18,
		MaxStandings:      22,
		DBMaxConns:        10,
	}
	var err error
	if cfg.ReadHeaderTimeout, err = duration("HTTP_READ_HEADER_TIMEOUT", cfg.ReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = duration("HTTP_READ_TIMEOUT", cfg.ReadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = duration("HTTP_WRITE_TIMEOUT", cfg.WriteTimeout); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = duration("HTTP_IDLE_TIMEOUT", cfg.IdleTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SourceTimeout, err = duration("SOURCE_TIMEOUT", cfg.SourceTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SourceMinInterval, err = duration("SOURCE_MIN_INTERVAL", cfg.SourceMinInterval); err != nil {
		return Config{}, err
	}
	if cfg.SyncInterval, err = duration("SYNC_INTERVAL", cfg.SyncInterval); err != nil {
		return Config{}, err
	}
	if cfg.SyncTimeout, err = duration("SYNC_TIMEOUT", cfg.SyncTimeout); err != nil {
		return Config{}, err
	}
	if cfg.StaleAfter, err = duration("STALE_AFTER", cfg.StaleAfter); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitRPS, err = floatEnv("RATE_LIMIT_RPS", cfg.RateLimitRPS); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitBurst, err = intEnv("RATE_LIMIT_BURST", cfg.RateLimitBurst); err != nil {
		return Config{}, err
	}
	if cfg.SourceRetries, err = intEnv("SOURCE_RETRIES", cfg.SourceRetries); err != nil {
		return Config{}, err
	}
	if cfg.SourceMaxBody, err = int64Env("SOURCE_MAX_BODY_BYTES", cfg.SourceMaxBody); err != nil {
		return Config{}, err
	}
	if cfg.SyncYear, err = intEnv("SYNC_YEAR", 0); err != nil {
		return Config{}, err
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SYNC_YEARS"))) {
	case "", "0":
		cfg.SyncAllYears = false
	case "all":
		cfg.SyncAllYears = true
	default:
		return Config{}, fmt.Errorf("SYNC_YEARS must be all or empty")
	}
	if cfg.MinMatches, err = intEnv("SYNC_MIN_MATCHES", cfg.MinMatches); err != nil {
		return Config{}, err
	}
	if cfg.MinStandings, err = intEnv("SYNC_MIN_STANDINGS", cfg.MinStandings); err != nil {
		return Config{}, err
	}
	if cfg.MaxStandings, err = intEnv("SYNC_MAX_STANDINGS", cfg.MaxStandings); err != nil {
		return Config{}, err
	}
	maxConns, err := intEnv("DB_MAX_CONNS", int(cfg.DBMaxConns))
	if err != nil {
		return Config{}, err
	}
	cfg.DBMaxConns = int32(maxConns)
	if cfg.TrustProxy, err = boolEnv("TRUST_PROXY", false); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) validate() error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") && !strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		return fmt.Errorf("DATABASE_URL must be a postgres URL")
	}
	if cfg.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	if _, err := catalog.Parse(cfg.SyncCompetition); err != nil {
		return fmt.Errorf("SYNC_COMPETITION: %w", err)
	}
	if cfg.SyncYear != 0 && (cfg.SyncYear < 1990 || cfg.SyncYear > 2100) {
		return fmt.Errorf("SYNC_YEAR is out of range")
	}
	if cfg.SyncInterval < 30*time.Second {
		return fmt.Errorf("SYNC_INTERVAL must be at least 30s")
	}
	if cfg.SourceMinInterval < 200*time.Millisecond {
		return fmt.Errorf("SOURCE_MIN_INTERVAL must be at least 200ms")
	}
	if cfg.SourceTimeout <= 0 || cfg.SyncTimeout <= 0 || cfg.StaleAfter <= 0 {
		return fmt.Errorf("timeouts must be positive")
	}
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.IdleTimeout <= 0 || cfg.ShutdownTimeout <= 0 || cfg.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("HTTP timeouts must be positive")
	}
	if cfg.RateLimitRPS <= 0 || cfg.RateLimitBurst < 1 {
		return fmt.Errorf("rate limit must be positive")
	}
	if cfg.SourceRetries < 0 || cfg.SourceRetries > 5 {
		return fmt.Errorf("SOURCE_RETRIES must be between 0 and 5")
	}
	if cfg.SourceMaxBody < 64<<10 || cfg.SourceMaxBody > 32<<20 {
		return fmt.Errorf("SOURCE_MAX_BODY_BYTES is out of range")
	}
	if cfg.MinMatches < 1 || cfg.MinStandings < 1 || cfg.MaxStandings < cfg.MinStandings {
		return fmt.Errorf("sync size limits are inconsistent")
	}
	if cfg.DBMaxConns < 1 || cfg.DBMaxConns > 50 {
		return fmt.Errorf("DB_MAX_CONNS must be between 1 and 50")
	}
	parsed, err := url.Parse(cfg.SourceBaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return fmt.Errorf("SOURCE_BASE_URL must be an absolute http(s) URL without user info")
	}
	if len(cfg.CORSOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS is empty")
	}
	return nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func intEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func int64Env(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func floatEnv(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}
