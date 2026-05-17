package entity

import (
	"time"

	"github.com/google/uuid"
)

// User is the core domain model for an authenticated account.
type User struct {
	ID           uuid.UUID
	Username     string
	FirstName    string
	LastName     string
	Email        string
	PasswordHash string
	IsVerified   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
