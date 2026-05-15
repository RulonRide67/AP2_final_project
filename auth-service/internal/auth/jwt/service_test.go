package jwt_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
)

type memoryRefreshStore struct {
	tokens map[string]uuid.UUID
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{tokens: make(map[string]uuid.UUID)}
}

func (m *memoryRefreshStore) Store(_ context.Context, token string, userID uuid.UUID, _ time.Duration) error {
	m.tokens[token] = userID
	return nil
}

func (m *memoryRefreshStore) GetUserID(_ context.Context, token string) (uuid.UUID, error) {
	id, ok := m.tokens[token]
	if !ok {
		return uuid.Nil, jwtsvc.ErrRefreshTokenNotFound
	}
	return id, nil
}

func (m *memoryRefreshStore) Delete(_ context.Context, token string) error {
	delete(m.tokens, token)
	return nil
}

func (m *memoryRefreshStore) RevokeAllForUser(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestIssueAndValidateAccessToken(t *testing.T) {
	t.Parallel()

	user := &entity.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	svc, err := jwtsvc.NewService(jwtsvc.Config{
		Secret:     "test-secret-key",
		Issuer:     "auth-service",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}, newMemoryRefreshStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	pair, err := svc.IssueTokenPair(context.Background(), user)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}

	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestRevokeRefreshToken(t *testing.T) {
	t.Parallel()

	user := &entity.User{ID: uuid.New(), Email: "user@example.com"}
	store := newMemoryRefreshStore()
	svc, err := jwtsvc.NewService(jwtsvc.Config{
		Secret:     "test-secret-key",
		Issuer:     "auth-service",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}, store)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	pair, err := svc.IssueTokenPair(context.Background(), user)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	if err := svc.RevokeRefreshToken(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}

	_, _, err = svc.RefreshAccessToken(context.Background(), pair.RefreshToken, user)
	if err == nil {
		t.Fatal("expected refresh to fail after revoke")
	}
}
