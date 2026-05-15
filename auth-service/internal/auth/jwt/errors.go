package jwt

import "errors"

var (
	// ErrInvalidToken is returned when a JWT is malformed or fails validation.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidTokenType is returned when token type does not match the expected use.
	ErrInvalidTokenType = errors.New("invalid token type")
	// ErrRefreshTokenNotFound is returned when a refresh token is missing or expired in Redis.
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)
