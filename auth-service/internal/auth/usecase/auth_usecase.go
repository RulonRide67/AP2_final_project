package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	"github.com/madiyar/final-project/auth-service/internal/domain/repository"
)

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(password, hash string) error
}

// JWTService issues and revokes access/refresh tokens.
type JWTService interface {
	IssueTokenPair(ctx context.Context, user *entity.User) (*jwtsvc.TokenPair, error)
	ValidateAccessToken(tokenString string) (*jwtsvc.Claims, error)
	UserIDFromRefreshToken(ctx context.Context, refreshToken string) (uuid.UUID, error)
	RotateRefreshToken(ctx context.Context, oldToken string, user *entity.User) (*jwtsvc.TokenPair, error)
	RevokeRefreshToken(ctx context.Context, refreshToken string) error
}

// AuthUseCase defines authentication business operations.
type AuthUseCase interface {
	Register(ctx context.Context, in RegisterInput) (*AuthResult, error)
	Login(ctx context.Context, in LoginInput) (*AuthResult, error)
	Logout(ctx context.Context, in LogoutInput) error
	RefreshToken(ctx context.Context, in RefreshTokenInput) (*AuthResult, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error)
}

// RegisterInput holds data required to create an account.
type RegisterInput struct {
	Username string
	Email    string
	Password string
}

// LoginInput holds credentials for authentication.
type LoginInput struct {
	Email    string
	Password string
}

// LogoutInput holds the refresh token to revoke.
type LogoutInput struct {
	RefreshToken string
}

// RefreshTokenInput holds the refresh token to rotate.
type RefreshTokenInput struct {
	RefreshToken string
}

// AuthResult is returned after successful register, login, or refresh.
type AuthResult struct {
	User   *entity.User
	Tokens *jwtsvc.TokenPair
}

type authUseCase struct {
	users    repository.UserRepository
	jwt      JWTService
	redis    *redis.Client
	hasher   PasswordHasher
}

// NewAuthUseCase creates an AuthUseCase implementation.
func NewAuthUseCase(
	users repository.UserRepository,
	jwtSvc JWTService,
	redisClient *redis.Client,
	hasher PasswordHasher,
) AuthUseCase {
	return &authUseCase{
		users:  users,
		jwt:    jwtSvc,
		redis:  redisClient,
		hasher: hasher,
	}
}

// Register creates a new user after checking email uniqueness and hashing the password.
func (uc *authUseCase) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if err := uc.ensureEmailAvailable(ctx, in.Email); err != nil {
		return nil, err
	}

	passwordHash, err := uc.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &entity.User{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: passwordHash,
		IsVerified:   false,
	}

	if err := uc.users.Create(ctx, user); err != nil {
		return nil, mapRepositoryError(err)
	}

	tokens, err := uc.jwt.IssueTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	return &AuthResult{User: user, Tokens: tokens}, nil
}

// Login authenticates a user and returns a new access/refresh token pair.
func (uc *authUseCase) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	user, err := uc.users.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	if err := uc.hasher.Compare(in.Password, user.PasswordHash); err != nil {
		return nil, ErrInvalidCredentials
	}

	tokens, err := uc.jwt.IssueTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	return &AuthResult{User: user, Tokens: tokens}, nil
}

// Logout revokes the given refresh token in Redis.
func (uc *authUseCase) Logout(ctx context.Context, in LogoutInput) error {
	if in.RefreshToken == "" {
		return ErrInvalidToken
	}

	if err := uc.jwt.RevokeRefreshToken(ctx, in.RefreshToken); err != nil {
		if errors.Is(err, jwtsvc.ErrRefreshTokenNotFound) {
			return ErrInvalidToken
		}
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RefreshToken validates the refresh token in Redis and issues a new token pair (rotation).
func (uc *authUseCase) RefreshToken(ctx context.Context, in RefreshTokenInput) (*AuthResult, error) {
	if in.RefreshToken == "" {
		return nil, ErrInvalidToken
	}

	userID, err := uc.jwt.UserIDFromRefreshToken(ctx, in.RefreshToken)
	if err != nil {
		if errors.Is(err, jwtsvc.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("resolve refresh token: %w", err)
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	tokens, err := uc.jwt.RotateRefreshToken(ctx, in.RefreshToken, user)
	if err != nil {
		if errors.Is(err, jwtsvc.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}

	return &AuthResult{User: user, Tokens: tokens}, nil
}

// GetProfile returns the user profile for the given user ID.
func (uc *authUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorized
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return user, nil
}

func (uc *authUseCase) ensureEmailAvailable(ctx context.Context, email string) error {
	_, err := uc.users.GetByEmail(ctx, email)
	if err == nil {
		return ErrUserAlreadyExists
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("check email: %w", err)
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrUnauthorized
	case errors.Is(err, repository.ErrEmailExists), errors.Is(err, repository.ErrUsernameExists):
		return ErrUserAlreadyExists
	default:
		return err
	}
}
