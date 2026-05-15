package jwt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const refreshKeyPrefix = "auth:refresh:"

// RefreshStore persists opaque refresh tokens (Redis-backed).
type RefreshStore interface {
	Store(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error
	GetUserID(ctx context.Context, token string) (uuid.UUID, error)
	Delete(ctx context.Context, token string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

// RedisRefreshStore stores refresh token hashes in Redis.
type RedisRefreshStore struct {
	client *redis.Client
}

// NewRedisRefreshStore creates a Redis-backed refresh token store.
func NewRedisRefreshStore(client *redis.Client) *RedisRefreshStore {
	return &RedisRefreshStore{client: client}
}

func (s *RedisRefreshStore) Store(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error {
	key := refreshKey(token)
	if err := s.client.Set(ctx, key, userID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

func (s *RedisRefreshStore) GetUserID(ctx context.Context, token string) (uuid.UUID, error) {
	val, err := s.client.Get(ctx, refreshKey(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, ErrRefreshTokenNotFound
		}
		return uuid.Nil, fmt.Errorf("get refresh token: %w", err)
	}

	userID, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse refresh token user id: %w", err)
	}
	return userID, nil
}

func (s *RedisRefreshStore) Delete(ctx context.Context, token string) error {
	if err := s.client.Del(ctx, refreshKey(token)).Err(); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// RevokeAllForUser removes refresh tokens for a user (best-effort scan by pattern).
func (s *RedisRefreshStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	pattern := refreshKeyPrefix + "*"
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := s.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if val == userID.String() {
			_ = s.client.Del(ctx, key).Err()
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("revoke refresh tokens for user: %w", err)
	}
	return nil
}

func refreshKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return refreshKeyPrefix + hex.EncodeToString(sum[:])
}
