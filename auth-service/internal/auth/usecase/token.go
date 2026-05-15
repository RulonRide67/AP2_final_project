package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	verifyTokenKeyPrefix = "auth:verify:"
	resetTokenKeyPrefix  = "auth:reset:"
)

func generateURLSafeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func actionTokenKey(prefix, token string) string {
	sum := sha256.Sum256([]byte(token))
	return prefix + hex.EncodeToString(sum[:])
}

func (uc *authUseCase) storeActionToken(ctx context.Context, prefix, token string, userID uuid.UUID, ttl time.Duration) error {
	if uc.redis == nil {
		return fmt.Errorf("redis client is not configured")
	}
	key := actionTokenKey(prefix, token)
	if err := uc.redis.Set(ctx, key, userID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("store action token: %w", err)
	}
	return nil
}

func (uc *authUseCase) consumeActionToken(ctx context.Context, prefix, token string) (uuid.UUID, error) {
	if uc.redis == nil {
		return uuid.Nil, fmt.Errorf("redis client is not configured")
	}

	key := actionTokenKey(prefix, token)
	userIDStr, err := uc.redis.GetDel(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, ErrInvalidToken
		}
		return uuid.Nil, fmt.Errorf("consume action token: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse user id from token: %w", err)
	}
	return userID, nil
}

func buildActionURL(baseURL, token string) string {
	sep := "?"
	if containsQuery(baseURL) {
		sep = "&"
	}
	return baseURL + sep + "token=" + token
}

func containsQuery(url string) bool {
	for i := 0; i < len(url); i++ {
		if url[i] == '?' {
			return true
		}
	}
	return false
}
