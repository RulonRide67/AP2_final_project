package usecase

import "errors"

var (
	// ErrInvalidCredentials is returned when email/password do not match.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserAlreadyExists is returned when registering with an existing email or username.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrUnauthorized is returned when the caller lacks permission for an action.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidToken is returned when a token is invalid or expired.
	ErrInvalidToken = errors.New("invalid token")
	// ErrAlreadyVerified is returned when verification is requested for a verified account.
	ErrAlreadyVerified = errors.New("email already verified")
	// ErrEmailSendFailed is returned when SMTP delivery fails.
	ErrEmailSendFailed = errors.New("failed to send email")
)
