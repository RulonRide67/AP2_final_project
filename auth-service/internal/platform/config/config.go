package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	jwtcfg "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/pkg/email"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	AppName           string
	AppEnv            string
	LogLevel          string
	GRPCHost          string
	GRPCPort          int
	HTTPMetricsHost   string
	HTTPMetricsPort   int
	HTTPPublicHost    string
	HTTPPublicPort    int
	DatabaseURL       string
	PostgresMaxConns  int32
	PostgresMinConns  int32
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	NATSURL           string
	NATSSubjectPrefix string
	SMTP              email.Config
	JWT               jwtcfg.Config
	UseCase           usecase.Config
}

// Load reads configuration from the environment (optionally from .env in the working directory).
func Load() (*Config, error) {
	if err := LoadEnvFile(".env"); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	accessTTL, err := durationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := durationEnv("JWT_REFRESH_TTL", 168*time.Hour)
	if err != nil {
		return nil, err
	}
	verifyTTL, err := durationEnv("VERIFICATION_TOKEN_TTL", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	resetTTL, err := durationEnv("PASSWORD_RESET_TOKEN_TTL", time.Hour)
	if err != nil {
		return nil, err
	}

	smtpPort, err := intEnv("SMTP_PORT", 587)
	if err != nil {
		return nil, err
	}
	grpcPort, err := intEnv("GRPC_PORT", 50051)
	if err != nil {
		return nil, err
	}
	metricsPort, err := intEnv("HTTP_METRICS_PORT", 9090)
	if err != nil {
		return nil, err
	}
	publicPort, err := intEnv("HTTP_PUBLIC_PORT", 8080)
	if err != nil {
		return nil, err
	}
	redisDB, err := intEnv("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	maxConns, err := int32Env("POSTGRES_MAX_CONNS", 25)
	if err != nil {
		return nil, err
	}
	minConns, err := int32Env("POSTGRES_MIN_CONNS", 5)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppName:         env("APP_NAME", "auth-service"),
		AppEnv:          env("APP_ENV", "development"),
		LogLevel:        env("LOG_LEVEL", "info"),
		GRPCHost:        env("GRPC_HOST", "0.0.0.0"),
		GRPCPort:        grpcPort,
		HTTPMetricsHost: env("HTTP_METRICS_HOST", "0.0.0.0"),
		HTTPMetricsPort: metricsPort,
		HTTPPublicHost:  env("HTTP_PUBLIC_HOST", "localhost"),
		HTTPPublicPort:  publicPort,
		DatabaseURL:     env("DATABASE_URL", ""),
		PostgresMaxConns: maxConns,
		PostgresMinConns: minConns,
		RedisAddr:       env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   env("REDIS_PASSWORD", ""),
		RedisDB:         redisDB,
		NATSURL:         env("NATS_URL", "nats://localhost:4222"),
		NATSSubjectPrefix: env("NATS_SUBJECT_PREFIX", "auth.events"),
		SMTP: email.Config{
			Host:     env("SMTP_HOST", "localhost"),
			Port:     smtpPort,
			Username: normalizeSMTPEmail(env("SMTP_USER", "")),
			Password: strings.ReplaceAll(env("SMTP_PASSWORD", ""), " ", ""),
			From:     normalizeSMTPEmail(env("SMTP_FROM", "noreply@auth.local")),
			FromName: env("SMTP_FROM_NAME", "Auth Service"),
			UseTLS:   boolEnv("SMTP_USE_TLS", smtpPort == 465),
		},
		JWT: jwtcfg.Config{
			Secret:     env("JWT_SECRET", ""),
			Issuer:     env("JWT_ISSUER", "auth-service"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		UseCase: usecase.Config{
			VerificationTokenTTL:  verifyTTL,
			PasswordResetTokenTTL: resetTTL,
			VerifyEmailBaseURL:    "",
			ResetPasswordBaseURL:  "",
		},
	}

	publicBase := fmt.Sprintf("http://%s", net.JoinHostPort(cfg.HTTPPublicHost, strconv.Itoa(cfg.HTTPPublicPort)))
	cfg.UseCase.VerifyEmailBaseURL = env("VERIFY_EMAIL_BASE_URL", publicBase+"/verify-email")
	cfg.UseCase.ResetPasswordBaseURL = env("RESET_PASSWORD_BASE_URL", publicBase+"/reset-password")

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = buildDatabaseURL()
	}
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.SMTP.Username != "" && !strings.Contains(cfg.SMTP.Username, "@") {
		return nil, fmt.Errorf("SMTP_USER must be an email address (e.g. name@gmail.com), not %q", cfg.SMTP.Username)
	}
	if cfg.SMTP.From != "" && !strings.Contains(cfg.SMTP.From, "@") {
		return nil, fmt.Errorf("SMTP_FROM must be an email address (e.g. name@gmail.com), not %q", cfg.SMTP.From)
	}

	return cfg, nil
}

// normalizeSMTPEmail strips accidental URL prefixes from SMTP credentials.
func normalizeSMTPEmail(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	return v
}

// GRPCAddr returns the gRPC listen address.
func (c *Config) GRPCAddr() string {
	return net.JoinHostPort(c.GRPCHost, strconv.Itoa(c.GRPCPort))
}

// MetricsAddr returns the Prometheus metrics HTTP listen address.
func (c *Config) MetricsAddr() string {
	return net.JoinHostPort(c.HTTPMetricsHost, strconv.Itoa(c.HTTPMetricsPort))
}

// PublicHTTPAddr returns the browser-facing HTTP listen address (0.0.0.0 for bind).
func (c *Config) PublicHTTPAddr() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.HTTPPublicPort))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func int32Env(key string, fallback int32) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return int32(n), nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func boolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func buildDatabaseURL() string {
	host := env("POSTGRES_HOST", "localhost")
	port := env("POSTGRES_PORT", "5432")
	user := env("POSTGRES_USER", "auth")
	password := env("POSTGRES_PASSWORD", "auth_secret")
	db := env("POSTGRES_DB", "auth_db")
	ssl := env("POSTGRES_SSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, db, ssl)
}
