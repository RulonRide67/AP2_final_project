package usecase

import "time"

// Config holds use-case level settings for tokens and email links.
type Config struct {
	VerificationTokenTTL time.Duration
	PasswordResetTokenTTL time.Duration
	VerifyEmailBaseURL    string
	ResetPasswordBaseURL  string
}

// DefaultConfig returns sensible defaults for development.
func DefaultConfig() Config {
	return Config{
		VerificationTokenTTL:  24 * time.Hour,
		PasswordResetTokenTTL: time.Hour,
		VerifyEmailBaseURL:    "http://localhost:3000/verify-email",
		ResetPasswordBaseURL:  "http://localhost:3000/reset-password",
	}
}
