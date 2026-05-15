package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	"github.com/madiyar/final-project/auth-service/internal/domain/repository"
)

type mockUserRepo struct {
	byEmail map[string]*entity.User
	byID    map[uuid.UUID]*entity.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		byEmail: make(map[string]*entity.User),
		byID:    make(map[uuid.UUID]*entity.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *entity.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if _, ok := m.byEmail[user.Email]; ok {
		return repository.ErrEmailExists
	}
	m.byEmail[user.Email] = user
	m.byID[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*entity.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*entity.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByUsername(context.Context, string) (*entity.User, error) {
	return nil, repository.ErrNotFound
}

func (m *mockUserRepo) Update(context.Context, *entity.User) error { return nil }

func (m *mockUserRepo) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }

func (m *mockUserRepo) SetEmailVerified(context.Context, uuid.UUID, bool) error { return nil }

func (m *mockUserRepo) Delete(context.Context, uuid.UUID) error { return nil }

type mockJWT struct {
	refresh map[string]uuid.UUID
	seq     int
}

func newMockJWT() *mockJWT {
	return &mockJWT{refresh: make(map[string]uuid.UUID)}
}

func (m *mockJWT) IssueTokenPair(_ context.Context, user *entity.User) (*jwtsvc.TokenPair, error) {
	m.seq++
	rt := fmt.Sprintf("refresh-%s-%d", user.ID, m.seq)
	m.refresh[rt] = user.ID
	return &jwtsvc.TokenPair{
		AccessToken:  fmt.Sprintf("access-%s-%d", user.ID, m.seq),
		RefreshToken: rt,
		ExpiresIn:    900,
		TokenType:    "Bearer",
	}, nil
}

func (m *mockJWT) ValidateAccessToken(token string) (*jwtsvc.Claims, error) {
	return &jwtsvc.Claims{}, nil
}

func (m *mockJWT) UserIDFromRefreshToken(_ context.Context, token string) (uuid.UUID, error) {
	id, ok := m.refresh[token]
	if !ok {
		return uuid.Nil, jwtsvc.ErrRefreshTokenNotFound
	}
	return id, nil
}

func (m *mockJWT) RotateRefreshToken(ctx context.Context, old string, user *entity.User) (*jwtsvc.TokenPair, error) {
	delete(m.refresh, old)
	return m.IssueTokenPair(ctx, user)
}

func (m *mockJWT) RevokeRefreshToken(_ context.Context, token string) error {
	if _, ok := m.refresh[token]; !ok {
		return jwtsvc.ErrRefreshTokenNotFound
	}
	delete(m.refresh, token)
	return nil
}

type mockHasher struct{}

func (mockHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func (mockHasher) Compare(password, hash string) error {
	if hash != "hashed:"+password {
		return errors.New("mismatch")
	}
	return nil
}

func TestRegisterAndLogin(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	jwtMock := newMockJWT()
	uc := usecase.NewAuthUseCase(repo, jwtMock, nil, mockHasher{})

	reg, err := uc.Register(context.Background(), usecase.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if reg.User.Email != "alice@example.com" || reg.Tokens.AccessToken == "" {
		t.Fatalf("unexpected register result: %+v", reg)
	}

	_, err = uc.Register(context.Background(), usecase.RegisterInput{
		Username: "alice2",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if !errors.Is(err, usecase.ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}

	login, err := uc.Login(context.Background(), usecase.LoginInput{
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if login.Tokens.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	_, err = uc.Login(context.Background(), usecase.LoginInput{
		Email:    "alice@example.com",
		Password: "wrong",
	})
	if !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRefreshAndLogout(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	jwtMock := newMockJWT()
	uc := usecase.NewAuthUseCase(repo, jwtMock, nil, mockHasher{})

	login, err := uc.Register(context.Background(), usecase.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	refreshed, err := uc.RefreshToken(context.Background(), usecase.RefreshTokenInput{
		RefreshToken: login.Tokens.RefreshToken,
	})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if refreshed.Tokens.RefreshToken == login.Tokens.RefreshToken {
		t.Fatal("expected rotated refresh token")
	}

	if err := uc.Logout(context.Background(), usecase.LogoutInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	}); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	_, err = uc.RefreshToken(context.Background(), usecase.RefreshTokenInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	})
	if !errors.Is(err, usecase.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken after logout, got %v", err)
	}
}

func TestGetProfile(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	jwtMock := newMockJWT()
	uc := usecase.NewAuthUseCase(repo, jwtMock, nil, mockHasher{})

	reg, _ := uc.Register(context.Background(), usecase.RegisterInput{
		Username: "carol",
		Email:    "carol@example.com",
		Password: "pass",
	})

	profile, err := uc.GetProfile(context.Background(), reg.User.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Email != "carol@example.com" {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	_, err = uc.GetProfile(context.Background(), uuid.Nil)
	if !errors.Is(err, usecase.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
