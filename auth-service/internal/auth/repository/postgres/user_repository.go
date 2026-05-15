package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	"github.com/madiyar/final-project/auth-service/internal/domain/repository"
)

const pgUniqueViolation = "23505"

// UserRepository implements repository.UserRepository with PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a PostgreSQL user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a new user. ID and timestamps may be set by the database.
func (r *UserRepository) Create(ctx context.Context, user *entity.User) error {
	const query = `
		INSERT INTO users (username, email, password_hash, is_verified)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err := r.pool.QueryRow(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.IsVerified,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return mapPgError(err)
	}
	return nil
}

// GetByID returns a user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	const query = `
		SELECT id, username, email, password_hash, is_verified, created_at, updated_at
		FROM users
		WHERE id = $1`

	return r.scanOne(ctx, query, id)
}

// GetByEmail returns a user by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	const query = `
		SELECT id, username, email, password_hash, is_verified, created_at, updated_at
		FROM users
		WHERE email = $1`

	return r.scanOne(ctx, query, email)
}

// GetByUsername returns a user by username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	const query = `
		SELECT id, username, email, password_hash, is_verified, created_at, updated_at
		FROM users
		WHERE username = $1`

	return r.scanOne(ctx, query, username)
}

// Update updates username and email for a user.
func (r *UserRepository) Update(ctx context.Context, user *entity.User) error {
	const query = `
		UPDATE users
		SET username = $2, email = $3, updated_at = (NOW() AT TIME ZONE 'UTC')
		WHERE id = $1
		RETURNING updated_at`

	err := r.pool.QueryRow(ctx, query, user.ID, user.Username, user.Email).Scan(&user.UpdatedAt)
	if err != nil {
		return mapPgError(err)
	}
	return nil
}

// UpdatePassword updates the password hash for a user.
func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	const query = `
		UPDATE users
		SET password_hash = $2, updated_at = (NOW() AT TIME ZONE 'UTC')
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// SetEmailVerified sets the email verification flag.
func (r *UserRepository) SetEmailVerified(ctx context.Context, id uuid.UUID, verified bool) error {
	const query = `
		UPDATE users
		SET is_verified = $2, updated_at = (NOW() AT TIME ZONE 'UTC')
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, verified)
	if err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// Delete removes a user by ID.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM users WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *UserRepository) scanOne(ctx context.Context, query string, arg any) (*entity.User, error) {
	row := r.pool.QueryRow(ctx, query, arg)
	user, err := scanUser(row)
	if err != nil {
		return nil, mapPgError(err)
	}
	return user, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*entity.User, error) {
	var u entity.User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.IsVerified,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	return &u, nil
}

func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		switch pgErr.ConstraintName {
		case "users_email_unique":
			return repository.ErrEmailExists
		case "users_username_unique":
			return repository.ErrUsernameExists
		}
	}

	return fmt.Errorf("postgres: %w", err)
}

// Ensure interface compliance at compile time.
var _ repository.UserRepository = (*UserRepository)(nil)
