package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error
}

// AuthUseCase defines authentication business operations.
type AuthUseCase interface {
	Register(ctx context.Context, in RegisterInput) (*AuthResult, error)
	Login(ctx context.Context, in LoginInput) (*AuthResult, error)
	Logout(ctx context.Context, in LogoutInput) error
	RefreshToken(ctx context.Context, in RefreshTokenInput) (*AuthResult, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error)
	SendVerificationEmail(ctx context.Context, in SendVerificationEmailInput) error
	VerifyEmail(ctx context.Context, in VerifyEmailInput) (*entity.User, error)
	ForgotPassword(ctx context.Context, in ForgotPasswordInput) error
	ResetPassword(ctx context.Context, in ResetPasswordInput) error
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

// SendVerificationEmailInput identifies the user to verify.
type SendVerificationEmailInput struct {
	Email  string
	UserID uuid.UUID
}

// VerifyEmailInput holds the verification token from the email link.
type VerifyEmailInput struct {
	Token string
}

// ForgotPasswordInput holds the account email for password reset.
type ForgotPasswordInput struct {
	Email string
}

// ResetPasswordInput holds the reset token and new password.
type ResetPasswordInput struct {
	Token       string
	NewPassword string
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
	mailer   EmailSender
	events   EventPublisher
	cfg      Config
}

// NewAuthUseCase creates an AuthUseCase implementation.
func NewAuthUseCase(
	users repository.UserRepository,
	jwtSvc JWTService,
	redisClient *redis.Client,
	hasher PasswordHasher,
	mailer EmailSender,
	events EventPublisher,
	cfg Config,
) AuthUseCase {
	if cfg.VerificationTokenTTL == 0 || cfg.PasswordResetTokenTTL == 0 {
		cfg = DefaultConfig()
	}
	if events == nil {
		events = NoopEventPublisher{}
	}
	return &authUseCase{
		users:  users,
		jwt:    jwtSvc,
		redis:  redisClient,
		hasher: hasher,
		mailer: mailer,
		events: events,
		cfg:    cfg,
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

	if err := uc.events.PublishUserCreated(ctx, user); err != nil {
		return nil, fmt.Errorf("publish user.created: %w", err)
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

// SendVerificationEmail generates a URL-safe token, stores it in Redis, and sends the email.
func (uc *authUseCase) SendVerificationEmail(ctx context.Context, in SendVerificationEmailInput) error {
	user, err := uc.resolveUserForEmailAction(ctx, in.Email, in.UserID)
	if err != nil {
		return err
	}

	if user.IsVerified {
		return ErrAlreadyVerified
	}

	token, err := generateURLSafeToken()
	if err != nil {
		return err
	}

	if err := uc.storeActionToken(ctx, verifyTokenKeyPrefix, token, user.ID, uc.cfg.VerificationTokenTTL); err != nil {
		return err
	}

	if uc.mailer == nil {
		return fmt.Errorf("%w: mailer not configured", ErrEmailSendFailed)
	}

	verifyURL := buildActionURL(uc.cfg.VerifyEmailBaseURL, token)
	if err := uc.mailer.SendVerificationEmail(ctx, user.Email, verifyURL); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
	}

	return nil
}

// VerifyEmail validates the token from Redis and marks the user as verified.
func (uc *authUseCase) VerifyEmail(ctx context.Context, in VerifyEmailInput) (*entity.User, error) {
	if strings.TrimSpace(in.Token) == "" {
		return nil, ErrInvalidToken
	}

	userID, err := uc.consumeActionToken(ctx, verifyTokenKeyPrefix, in.Token)
	if err != nil {
		return nil, err
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	if user.IsVerified {
		return user, nil
	}

	if err := uc.users.SetEmailVerified(ctx, user.ID, true); err != nil {
		return nil, mapRepositoryError(err)
	}
	user.IsVerified = true

	if err := uc.events.PublishUserVerified(ctx, user); err != nil {
		return nil, fmt.Errorf("publish user.verified: %w", err)
	}

	return user, nil
}

// ForgotPassword sends a reset link when the account exists (always succeeds from caller view).
func (uc *authUseCase) ForgotPassword(ctx context.Context, in ForgotPasswordInput) error {
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return nil
	}

	user, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	token, err := generateURLSafeToken()
	if err != nil {
		return err
	}

	if err := uc.storeActionToken(ctx, resetTokenKeyPrefix, token, user.ID, uc.cfg.PasswordResetTokenTTL); err != nil {
		return err
	}

	if uc.mailer == nil {
		return fmt.Errorf("%w: mailer not configured", ErrEmailSendFailed)
	}

	resetURL := buildActionURL(uc.cfg.ResetPasswordBaseURL, token)
	if err := uc.mailer.SendPasswordResetEmail(ctx, user.Email, resetURL); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
	}

	return nil
}

// ResetPassword validates the reset token and updates the user's password.
func (uc *authUseCase) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	if strings.TrimSpace(in.Token) == "" {
		return ErrInvalidToken
	}

	userID, err := uc.consumeActionToken(ctx, resetTokenKeyPrefix, in.Token)
	if err != nil {
		return err
	}

	passwordHash, err := uc.hasher.Hash(in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := uc.users.UpdatePassword(ctx, userID, passwordHash); err != nil {
		return mapRepositoryError(err)
	}

	if err := uc.jwt.RevokeAllRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revoke sessions after password reset: %w", err)
	}

	return nil
}

func (uc *authUseCase) resolveUserForEmailAction(ctx context.Context, email string, userID uuid.UUID) (*entity.User, error) {
	if userID != uuid.Nil {
		user, err := uc.users.GetByID(ctx, userID)
		if err != nil {
			return nil, mapRepositoryError(err)
		}
		return user, nil
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return nil, ErrUnauthorized
	}

	user, err := uc.users.GetByEmail(ctx, email)
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
