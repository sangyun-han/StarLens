// Package config loads StarLens server configuration from the environment.
//
// Every value has a sane local-development default so that `go run ./cmd/server`
// works against a stock StarRocks all-in-one container with no setup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Config is the fully resolved server configuration.
type Config struct {
	StarRocks StarRocksConfig
	Server    ServerConfig
	Alert     AlertConfig
	Query     QueryConfig
}

// QueryConfig bounds the SQL worksheet's ad-hoc query execution.
type QueryConfig struct {
	// ReadOnly restricts statements to SELECT/SHOW/DESCRIBE/EXPLAIN/WITH.
	// StarLens ships without authentication, so writes are off by default.
	ReadOnly bool
	// MaxRows is the hard cap on returned rows; client requests are clamped.
	MaxRows int
	// Timeout bounds one worksheet execution end to end.
	Timeout time.Duration
}

// StarRocksConfig describes how to reach StarRocks over the MySQL protocol.
type StarRocksConfig struct {
	// DSN is a normalized go-sql-driver/mysql DSN aimed at an FE query port.
	DSN string
	// Addr is the host:port the DSN points at, kept for logs and error messages.
	Addr string
	// Database is the DSN's default database, restored after scoped worksheet
	// executions.
	Database string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	// QueryTimeout bounds a single metadata query (SHOW FRONTENDS, ...).
	QueryTimeout time.Duration
	// DialTimeout bounds the initial TCP handshake with an FE.
	DialTimeout time.Duration
}

// ServerConfig describes the HTTP listener.
type ServerConfig struct {
	Port           string
	AllowedOrigins []string
	GinMode        string
}

// AlertConfig describes the alerting subsystem: how often rules are evaluated,
// how long repeats of the same condition are suppressed, where alerts are
// delivered, and the routine load rule thresholds.
type AlertConfig struct {
	// Enabled turns the background evaluation loop on/off. Alerts endpoints
	// stay mounted either way.
	Enabled      bool
	PollInterval time.Duration
	// Cooldown suppresses repeats of the same alert key.
	Cooldown time.Duration

	// WebhookURL, when set, registers a webhook notifier.
	WebhookURL string
	// WebhookFormat is "generic" (full alert JSON) or "slack" ({"text": ...}).
	WebhookFormat string

	// ErrorRowsRatio fires when errorRows/totalRows exceeds this fraction;
	// <= 0 disables the rule.
	ErrorRowsRatio float64
	// ErrorRowsMinTotal is the minimum consumed rows before the ratio rule
	// applies.
	ErrorRowsMinTotal int64
	// MaxOffsetLag fires when a job's approximate offset lag exceeds this many
	// messages; <= 0 disables the rule.
	MaxOffsetLag int64

	// UIEditable gates PUT /api/v1/alerts/config. Until StarLens has
	// authentication, disabling it stops dashboard visitors from redirecting
	// alerts; the environment then remains the only writer.
	UIEditable bool
	// OverrideFile is where UI-authored overrides persist as JSON. Environment
	// variables stay the defaults; the file wins where it sets a field.
	OverrideFile string
}

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	sr, err := loadStarRocks()
	if err != nil {
		return Config{}, err
	}

	alertCfg, err := loadAlert()
	if err != nil {
		return Config{}, err
	}

	return Config{
		StarRocks: sr,
		Server: ServerConfig{
			Port:           envString("SERVER_PORT", "8080"),
			AllowedOrigins: envStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
			GinMode:        envString("GIN_MODE", "debug"),
		},
		Alert: alertCfg,
		Query: QueryConfig{
			ReadOnly: envBool("QUERY_READ_ONLY", true),
			MaxRows:  envInt("QUERY_MAX_ROWS", 1000),
			Timeout:  envDuration("QUERY_TIMEOUT", time.Minute),
		},
	}, nil
}

func loadAlert() (AlertConfig, error) {
	cfg := AlertConfig{
		Enabled:           envBool("ALERT_ENABLED", true),
		PollInterval:      envDuration("ALERT_POLL_INTERVAL", 30*time.Second),
		Cooldown:          envDuration("ALERT_COOLDOWN", 10*time.Minute),
		WebhookURL:        strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL")),
		WebhookFormat:     envString("ALERT_WEBHOOK_FORMAT", "generic"),
		ErrorRowsRatio:    envFloat("ALERT_ERROR_ROWS_RATIO", 0.01),
		ErrorRowsMinTotal: envInt64("ALERT_ERROR_ROWS_MIN_TOTAL", 10_000),
		MaxOffsetLag:      envInt64("ALERT_MAX_OFFSET_LAG", 0),
		UIEditable:        envBool("ALERT_CONFIG_UI", true),
		OverrideFile:      envString("ALERT_CONFIG_FILE", "starlens-alerts.json"),
	}

	if cfg.WebhookFormat != "generic" && cfg.WebhookFormat != "slack" {
		return AlertConfig{}, fmt.Errorf(
			"config: ALERT_WEBHOOK_FORMAT must be \"generic\" or \"slack\", got %q", cfg.WebhookFormat)
	}
	return cfg, nil
}

func loadStarRocks() (StarRocksConfig, error) {
	var (
		maxOpen  = envInt("STARROCKS_MAX_OPEN_CONNS", 25)
		maxIdle  = envInt("STARROCKS_MAX_IDLE_CONNS", 10)
		lifetime = envDuration("STARROCKS_CONN_MAX_LIFETIME", 30*time.Minute)
		queryTO  = envDuration("STARROCKS_QUERY_TIMEOUT", 10*time.Second)
		dialTO   = envDuration("STARROCKS_DIAL_TIMEOUT", 5*time.Second)
	)

	// An idle pool larger than the open limit is silently clamped by database/sql,
	// which hides a misconfiguration — surface it instead.
	if maxIdle > maxOpen {
		return StarRocksConfig{}, fmt.Errorf(
			"config: STARROCKS_MAX_IDLE_CONNS (%d) must not exceed STARROCKS_MAX_OPEN_CONNS (%d)", maxIdle, maxOpen)
	}

	dsn, err := resolveDSN(dialTO)
	if err != nil {
		return StarRocksConfig{}, err
	}

	// Re-parse to echo back exactly what the driver will dial.
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return StarRocksConfig{}, fmt.Errorf("config: invalid StarRocks DSN: %w", err)
	}

	return StarRocksConfig{
		DSN:             dsn,
		Addr:            parsed.Addr,
		Database:        parsed.DBName,
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxLifetime: lifetime,
		QueryTimeout:    queryTO,
		DialTimeout:     dialTO,
	}, nil
}

// resolveDSN prefers STARROCKS_DSN and otherwise assembles a DSN from parts.
// Either way the result is round-tripped through mysql.Config so credentials are
// escaped correctly and connection timeouts are always present.
func resolveDSN(dialTimeout time.Duration) (string, error) {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"

	if raw := strings.TrimSpace(os.Getenv("STARROCKS_DSN")); raw != "" {
		parsed, err := mysql.ParseDSN(raw)
		if err != nil {
			return "", fmt.Errorf("config: STARROCKS_DSN is not a valid MySQL DSN (want user:pass@tcp(host:9030)/db): %w", err)
		}
		cfg = parsed
	} else {
		cfg.User = envString("STARROCKS_USER", "root")
		cfg.Passwd = os.Getenv("STARROCKS_PASSWORD")
		cfg.Addr = envString("STARROCKS_HOST", "127.0.0.1") + ":" + envString("STARROCKS_PORT", "9030")
		cfg.DBName = envString("STARROCKS_DATABASE", "information_schema")
	}

	if cfg.Addr == "" {
		return "", fmt.Errorf("config: StarRocks address is empty; set STARROCKS_DSN or STARROCKS_HOST")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = dialTimeout
	}
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}
	// StarRocks reports SHOW/metadata timestamps as strings and happily returns
	// "0000-00-00 00:00:00" for nodes that never started, which the driver cannot
	// convert to time.Time. Everything is read as text, so leave ParseTime off.
	cfg.ParseTime = false

	return cfg.FormatDSN(), nil
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envStringSlice(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

// envFloat reads a float env var. Unlike envInt it accepts zero and negative
// values: rule thresholds use <= 0 to mean "disabled".
func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

// envInt64 reads an int64 env var, accepting zero and negative values for the
// same "disabled" convention as envFloat.
func envInt64(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
