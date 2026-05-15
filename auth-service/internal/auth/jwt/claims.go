package jwt

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const tokenTypeAccess = "access"

// Claims are embedded in signed JWT access and refresh tokens.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Type   string    `json:"type"`
	jwt.RegisteredClaims
}
