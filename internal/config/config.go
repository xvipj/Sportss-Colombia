package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config contiene toda la configuración cargada desde el entorno.
type Config struct {
	AppName            string
	AppEnv             string
	Port               string
	BaseURL            string
	SecretKey          string
	JWTSecret          string
	CookieName         string
	CookieSecure       bool
	SQLitePath         string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	MPAccessToken      string
	MPPublicKey        string
	MPWebhookSecret    string
	MPSandbox          bool
	CommissionPercent  float64
	BookingLockMinutes int
	PendingMinutes     int
	CancellationHours  int
	UploadMaxMB        int64
	Timezone           string
	Location           *time.Location
	StorageDir         string
	InvoiceDir         string
	UploadDir          string
	TemplateDir        string
	StaticDir          string
}

// Load lee .env (si existe) y valida valores esenciales.
func Load() (*Config, error) {
	_ = godotenv.Load()

	locName := env("APP_TIMEZONE", "America/Bogota")
	loc, err := time.LoadLocation(locName)
	if err != nil {
		loc = time.FixedZone("COT", -5*60*60)
	}

	cfg := &Config{
		AppName:            env("APP_NAME", "Sportss Colombia"),
		AppEnv:             env("APP_ENV", "development"),
		Port:               env("APP_PORT", "8080"),
		BaseURL:            strings.TrimRight(env("APP_BASE_URL", "http://localhost:8080"), "/"),
		SecretKey:          env("APP_SECRET", "sportss-dev-secret-change-me"),
		JWTSecret:          env("JWT_SECRET", env("APP_SECRET", "sportss-dev-secret-change-me")),
		CookieName:         env("COOKIE_NAME", "sportss_token"),
		SQLitePath:         env("SQLITE_PATH", "storage/sportss.db"),
		RedisAddr:          env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      env("REDIS_PASSWORD", ""),
		RedisDB:            envInt("REDIS_DB", 0),
		MPAccessToken:      env("MP_ACCESS_TOKEN", ""),
		MPPublicKey:        env("MP_PUBLIC_KEY", ""),
		MPWebhookSecret:    env("MP_WEBHOOK_SECRET", ""),
		MPSandbox:          envBool("MP_SANDBOX", true),
		CommissionPercent:  envFloat("COMMISSION_PERCENT", 10),
		BookingLockMinutes: envInt("BOOKING_LOCK_MINUTES", 10),
		PendingMinutes:     envInt("PENDING_EXPIRATION_MINUTES", 10),
		CancellationHours:  envInt("CANCELLATION_HOURS", 24),
		UploadMaxMB:        int64(envInt("UPLOAD_MAX_MB", 5)),
		Timezone:           locName,
		Location:           loc,
		StorageDir:         env("STORAGE_DIR", "storage"),
		InvoiceDir:         env("INVOICE_DIR", filepath.Join("storage", "invoices")),
		UploadDir:          env("UPLOAD_DIR", filepath.Join("web", "static", "uploads")),
		TemplateDir:        env("TEMPLATE_DIR", filepath.Join("web", "templates")),
		StaticDir:          env("STATIC_DIR", filepath.Join("web", "static")),
	}
	cfg.CookieSecure = envBool("COOKIE_SECURE", cfg.AppEnv == "production")

	if cfg.CommissionPercent < 0 || cfg.CommissionPercent > 100 {
		return nil, fmt.Errorf("COMMISSION_PERCENT debe estar entre 0 y 100")
	}
	if cfg.BookingLockMinutes < 1 {
		cfg.BookingLockMinutes = 10
	}
	if cfg.PendingMinutes < 1 {
		cfg.PendingMinutes = 10
	}
	if cfg.CancellationHours < 0 {
		cfg.CancellationHours = 24
	}
	if cfg.UploadMaxMB < 1 || cfg.UploadMaxMB > 20 {
		cfg.UploadMaxMB = 5
	}

	for _, dir := range []string{filepath.Dir(cfg.SQLitePath), cfg.StorageDir, cfg.InvoiceDir, cfg.UploadDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("crear directorio %s: %w", dir, err)
		}
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

// PaymentMockEnabled indica si se usará el checkout local de desarrollo.
func (c *Config) PaymentMockEnabled() bool { return strings.TrimSpace(c.MPAccessToken) == "" }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(env(key, ""), 64)
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(env(key, ""))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "si" || v == "sí"
}
