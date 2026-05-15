package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	SetEmailVerified(ctx context.Context, id uuid.UUID, verified bool) error
	Delete(ctx context.Context, id uuid.UUID) error
}
