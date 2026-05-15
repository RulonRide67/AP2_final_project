package jwt

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
)

// TokenPair contains access and refresh tokens for a session.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	TokenType    string
}

// Service issues and validates JWT access tokens and opaque refresh tokens.
type Service struct {
	secret       []byte
	issuer       string
	accessTTL    time.Duration
	refreshTTL   time.Duration
	refreshStore RefreshStore
}

// NewService creates a JWT service.
func NewService(cfg Config, refreshStore RefreshStore) (*Service, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	if cfg.AccessTTL <= 0 || cfg.RefreshTTL <= 0 {
		return nil, fmt.Errorf("jwt token TTLs must be positive")
	}
	if refreshStore == nil {
		return nil, fmt.Errorf("refresh store is required")
	}

	return &Service{
		secret:       []byte(cfg.Secret),
		issuer:       cfg.Issuer,
		accessTTL:    cfg.AccessTTL,
		refreshTTL:   cfg.RefreshTTL,
		refreshStore: refreshStore,
	}, nil
}

// IssueTokenPair creates a short-lived access JWT and a long-lived refresh token stored in Redis.
func (s *Service) IssueTokenPair(ctx context.Context, user *entity.User) (*TokenPair, error) {
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	if err := s.refreshStore.Store(ctx, refreshToken, user.ID, s.refreshTTL); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// ValidateAccessToken parses and validates an access JWT and returns its claims.
func (s *Service) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type != tokenTypeAccess {
		return nil, ErrInvalidTokenType
	}
	return claims, nil
}

// RefreshAccessToken validates a refresh token in Redis and issues a new access token.
func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string, user *entity.User) (string, int64, error) {
	userID, err := s.refreshStore.GetUserID(ctx, refreshToken)
	if err != nil {
		return "", 0, err
	}
	if userID != user.ID {
		return "", 0, ErrInvalidToken
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return "", 0, err
	}

	return accessToken, int64(s.accessTTL.Seconds()), nil
}

// RotateRefreshToken replaces the refresh token (logout old, store new).
func (s *Service) RotateRefreshToken(ctx context.Context, oldToken string, user *entity.User) (*TokenPair, error) {
	if err := s.refreshStore.Delete(ctx, oldToken); err != nil {
		return nil, err
	}
	return s.IssueTokenPair(ctx, user)
}

// UserIDFromRefreshToken returns the user ID bound to a refresh token in Redis.
func (s *Service) UserIDFromRefreshToken(ctx context.Context, refreshToken string) (uuid.UUID, error) {
	return s.refreshStore.GetUserID(ctx, refreshToken)
}

// RevokeRefreshToken removes a refresh token from Redis (logout).
func (s *Service) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	return s.refreshStore.Delete(ctx, refreshToken)
}

// RevokeAllRefreshTokens removes all refresh tokens for a user.
func (s *Service) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	return s.refreshStore.RevokeAllForUser(ctx, userID)
}

func (s *Service) generateAccessToken(user *entity.User) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Type:   tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (s *Service) parseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
