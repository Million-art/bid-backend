package env

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort          string
	CORSOrigins      string // comma-separated origins for CORS
	DatabaseURL      string
	DBSSLMode        string
	DBChannelBinding string
	// R2 presigned upload configuration
	R2AccountID                      string
	R2AccessKeyID                    string
	R2SecretAccessKey                string
	R2Bucket                         string
	R2Prefix                         string
	R2PublicDomain                   string // e.g. cdn.carramarket.com
	ResendAPIKey                     string
	ResendFromEmail                  string
	JWTAccessSecret                  string
	JWTRefreshSecret                 string
	CSRFSecret                       string
	MaxUploadSizeMB                  int
	RedisHost                        string
	RedisPort                        string
	RedisPassword                    string
	SeedAdminEmail                   string
	SeedAdminPassword                string
	ParticipantAppURL                string
	CookieSecure                     bool
	CookieSameSite                   string
	CookieDomain                     string
	AppName                          string
	SupportEmail                     string
	ExposeDebugMetrics               bool
	GoogleClientID                   string
	TelegramBotToken                 string // BOT_TOKEN or TELEGRAM_BOT_TOKEN
	TelegramBotUsername              string // BOT_USERNAME or TELEGRAM_BOT_USERNAME
	TelegramMiniAppShortName         string // TELEGRAM_MINI_APP_SHORT_NAME (optional BotFather /newapp short name)
	TelegramChannelID                string // CHANNEL_ID  channel/supergroup for auction posts
	TelegramAdminID                  string // ADMIN_ID  optional admin chat for failure DMs

	// Chapa Payment Gateway
	ChapaPublicKey     string
	ChapaSecretKey     string
	ChapaEncryptionKey string
	ChapaWebhookSecret string
	ChapaAPIURL        string
	CallbackURL        string
	ChapaBaseURL       string

	// AfroMessage SMS
	AfroMessageAPIKey       string
	AfroMessageIdentifierID string
	AfroMessageSenderName   string
	AfroMessageBaseURL      string

	LogLevel  string // LogLevel: debug | info | warn | error (default: info)
	LogFormat string // LogFormat: json | text (default: json)

	// Sentry error tracking
	SentryDSN              string
	SentryTracesSampleRate float64
	SentryEnvironment      string
}

const (
	minJWTSecretLength  = 32
	minCSRFSecretLength = 32
	csrfDevDefault      = "default_csrf_secret_must_be_at_least_32_chars_long"
)

func (c *Config) Validate() error {
	if len(c.JWTAccessSecret) < minJWTSecretLength || c.JWTAccessSecret == "change-me-in-production-min-32-chars-required" || c.JWTAccessSecret == "default_access_secret_must_be_at_least_32_chars_long" {
		if c.JWTAccessSecret != "default_access_secret_must_be_at_least_32_chars_long" {
			return fmt.Errorf("JWT_ACCESS_SECRET must be at least %d characters", minJWTSecretLength)
		}
	}
	if len(c.JWTRefreshSecret) < minJWTSecretLength || c.JWTRefreshSecret == "change-me-in-production-min-32-chars-required" || c.JWTRefreshSecret == "default_refresh_secret_must_be_at_least_32_chars_long" {
		if c.JWTRefreshSecret != "default_refresh_secret_must_be_at_least_32_chars_long" {
			return fmt.Errorf("JWT_REFRESH_SECRET must be at least %d characters", minJWTSecretLength)
		}
	}
	if c.CSRFSecret != csrfDevDefault && (len(c.CSRFSecret) < minCSRFSecretLength || c.CSRFSecret == "change-me-in-production-min-32-chars-required") {
		return fmt.Errorf("CSRF_SECRET must be at least %d characters", minCSRFSecretLength)
	}
	if c.CSRFSecret == c.JWTAccessSecret || c.CSRFSecret == c.JWTRefreshSecret {
		return fmt.Errorf("CSRF_SECRET must differ from JWT secrets")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DB_URL is required")
	}
	return nil
}

func loadAdminDotEnv() {
	cwd, _ := os.Getwd()
	slog.Info("env: searching for .env", "cwd", cwd)

	// Try a few common locations relative to the working directory
	paths := []string{
		".env",
		"admin-backend/.env",
		"../../.env", // useful when running from cmd/tool
	}

	for _, p := range paths {
		if err := godotenv.Load(p); err == nil {
			slog.Info("env: loaded config", "path", p)
			return
		}
	}
	slog.Warn("env: no .env file found in search paths")
}

func LoadConfig() *Config {
	loadAdminDotEnv()

	return &Config{
		AppPort:                          sanitizePort(getEnv("APP_PORT", "8080")),
		CORSOrigins:                      getEnv("CORS_ORIGINS", ""),
		DatabaseURL:                      requireEnv("DB_URL"),
		DBSSLMode:                        getEnv("DB_SSLMODE", "require"),
		DBChannelBinding:                 getEnv("DB_CHANNEL_BINDING", ""),
		R2AccountID:                      getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:                    getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey:                getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:                         getEnv("R2_BUCKET", ""),
		R2Prefix:                         getEnv("R2_PREFIX", ""),
		R2PublicDomain:                   getEnv("R2_PUBLIC_DOMAIN", ""),
		ResendAPIKey:                     getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:                  getEnv("RESEND_FROM_EMAIL", "noreply@carramarket.com"),
		JWTAccessSecret:                  getEnv("JWT_ACCESS_SECRET", "default_access_secret_must_be_at_least_32_chars_long"),
		JWTRefreshSecret:                 getEnv("JWT_REFRESH_SECRET", "default_refresh_secret_must_be_at_least_32_chars_long"),
		CSRFSecret:                       getEnv("CSRF_SECRET", csrfDevDefault),
		MaxUploadSizeMB:                  getEnvInt("MAX_UPLOAD_SIZE_MB", 20),
		RedisHost:                        getEnv("REDIS_HOST", "localhost"),
		RedisPort:                        getEnv("REDIS_PORT", "6379"),
		RedisPassword:                    getEnv("REDIS_PASSWORD", ""),
		SeedAdminEmail:                   getEnv("SEED_ADMIN_EMAIL", "admin@carramarket.com"),
		SeedAdminPassword:                getEnv("SEED_ADMIN_PASSWORD", "password123"),
		ParticipantAppURL:                getEnv("PARTICIPANT_APP_URL", "http://localhost:3000"),
		CookieSecure:                     getEnvBool("COOKIE_SECURE", false),
		CookieSameSite:                   getEnv("COOKIE_SAME_SITE", "Lax"),
		CookieDomain:                     getEnv("COOKIE_DOMAIN", "localhost"),
		AppName:                          getEnv("APP_NAME", "Carra Market"),
		SupportEmail:                     getEnv("SUPPORT_EMAIL", "support@carramarket.com"),
		ExposeDebugMetrics:               getEnvBool("EXPOSE_DEBUG_METRICS", false),
		GoogleClientID:                   getEnv("GOOGLE_CLIENT_ID", "510278494876-bkmbkv9hfuelp0a5pv8lcqpd32ondgst.apps.googleusercontent.com"),
		TelegramBotToken:                 getEnvFallback("BOT_TOKEN", "TELEGRAM_BOT_TOKEN", ""),
		TelegramBotUsername:              strings.TrimPrefix(getEnvFallback("TELEGRAM_BOT_USERNAME", "BOT_USERNAME", ""), "@"),
		TelegramMiniAppShortName:         strings.Trim(getEnvFallback("TELEGRAM_MINI_APP_SHORT_NAME", "MINI_APP_SHORT_NAME", ""), "/ "),
		TelegramChannelID:                getEnvFallback("CHANNEL_ID", "TELEGRAM_CHANNEL_ID", ""),
		TelegramAdminID:                  getEnvFallback("ADMIN_ID", "TELEGRAM_ADMIN_ID", ""),
		ChapaPublicKey:                   getEnv("CHAPA_PUBLIC_KEY", ""),
		ChapaSecretKey:                   getEnv("CHAPA_SECRET_KEY", ""),
		ChapaEncryptionKey:               getEnv("CHAPA_ENCRYPTION_KEY", ""),
		ChapaWebhookSecret:               getEnv("CHAPA_WEBHOOK_SECRET", ""),
		ChapaAPIURL:                      getEnv("CHAPA_API_URL", ""),
		CallbackURL:                      getEnv("CALLBACK_URL", ""),
		ChapaBaseURL:                     getEnv("CHAPA_BASE_URL", "https://api.chapa.co/v1"),
		AfroMessageAPIKey:                getEnvFallback("AFRO_MESSAGE_API_KEY", "AFROMESSAGE_API_KEY", ""),
		AfroMessageIdentifierID:          getEnvFallback("AFRO_MESSAGE_IDENTIFIER", "AFROMESSAGE_IDENTIFIER_ID", ""),
		AfroMessageSenderName:            getEnvFallback("AFRO_MESSAGE_SENDER_ID", "AFROMESSAGE_SENDER_NAME", "Carra"),
		AfroMessageBaseURL:               getEnvFallback("AFRO_MESSAGE_BASE_URL", "AFROMESSAGE_BASE_URL", "https://api.afromessage.com/api/send"),
		LogLevel:                         getEnv("LOG_LEVEL", "info"),
		LogFormat:                        getEnv("LOG_FORMAT", "json"),
		SentryDSN:                        getEnv("SENTRY_DSN", ""),
		SentryTracesSampleRate:           getEnvFloat("SENTRY_TRACES_SAMPLE_RATE", 0.2),
		SentryEnvironment:                getEnv("SENTRY_ENVIRONMENT", "development"),
	}
}

func requireEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		slog.Error("FATAL: Environment variable is required but not set", "key", key)
		os.Exit(1)
	}
	return value
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvFallback tries primaryKey first, then secondaryKey, then returns the default.
// This allows supporting multiple naming conventions for the same config.
func getEnvFallback(primaryKey, secondaryKey, fallback string) string {
	if value, exists := os.LookupEnv(primaryKey); exists && strings.TrimSpace(value) != "" {
		return value
	}
	if value, exists := os.LookupEnv(secondaryKey); exists && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return fallback
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		slog.Error("FATAL: Environment variable must be an integer", "key", key, "value", s)
		os.Exit(1)
	}
	return i
}

func getEnvFloat(key string, fallback float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		slog.Error("FATAL: Environment variable must be a float", "key", key, "value", s)
		os.Exit(1)
	}
	return f
}

func getEnvBool(key string, fallback bool) bool {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return fallback
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fallback
	}
	return b
}

func sanitizePort(p string) string {
	if strings.Contains(p, ":") {
		parts := strings.Split(p, ":")
		return parts[len(parts)-1]
	}
	return p
}