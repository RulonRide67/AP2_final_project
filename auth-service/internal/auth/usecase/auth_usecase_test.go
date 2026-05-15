package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

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

func (m *mockUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	u, ok := m.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.PasswordHash = hash
	return nil
}

func (m *mockUserRepo) SetEmailVerified(_ context.Context, id uuid.UUID, verified bool) error {
	u, ok := m.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.IsVerified = verified
	return nil
}

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

func (m *mockJWT) ValidateAccessToken(string) (*jwtsvc.Claims, error) {
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

func (m *mockJWT) RevokeAllRefreshTokens(context.Context, uuid.UUID) error { return nil }

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

type mockMailer struct {
	lastVerifyURL string
	lastResetURL  string
}

func (m *mockMailer) SendVerificationEmail(_ context.Context, _, verifyURL string) error {
	m.lastVerifyURL = verifyURL
	return nil
}

func (m *mockMailer) SendPasswordResetEmail(_ context.Context, _, resetURL string) error {
	m.lastResetURL = resetURL
	return nil
}

type recordingPublisher struct {
	created  int
	verified int
}

func (p *recordingPublisher) PublishUserCreated(context.Context, *entity.User) error {
	p.created++
	return nil
}

func (p *recordingPublisher) PublishUserVerified(context.Context, *entity.User) error {
	p.verified++
	return nil
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newTestUseCase(t *testing.T, repo *mockUserRepo, jwt *mockJWT, mailer usecase.EmailSender, events usecase.EventPublisher) usecase.AuthUseCase {
	t.Helper()
	return usecase.NewAuthUseCase(repo, jwt, newTestRedis(t), mockHasher{}, mailer, events, usecase.DefaultConfig())
}

func TestRegisterAndLogin(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	jwtMock := newMockJWT()
	pub := &recordingPublisher{}
	uc := newTestUseCase(t, repo, jwtMock, nil, pub)

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
	if pub.created != 1 {
		t.Fatalf("expected user.created event, got %d", pub.created)
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
	uc := newTestUseCase(t, repo, jwtMock, nil, nil)

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
	uc := newTestUseCase(t, repo, jwtMock, nil, nil)

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

func TestEmailVerificationFlow(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	mailer := &mockMailer{}
	pub := &recordingPublisher{}
	uc := newTestUseCase(t, repo, newMockJWT(), mailer, pub)

	reg, err := uc.Register(context.Background(), usecase.RegisterInput{
		Username: "dave",
		Email:    "dave@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := uc.SendVerificationEmail(context.Background(), usecase.SendVerificationEmailInput{
		Email: "dave@example.com",
	}); err != nil {
		t.Fatalf("SendVerificationEmail: %v", err)
	}
	if mailer.lastVerifyURL == "" {
		t.Fatal("expected verification URL in email")
	}

	token := extractTokenParam(mailer.lastVerifyURL)
	user, err := uc.VerifyEmail(context.Background(), usecase.VerifyEmailInput{Token: token})
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if !user.IsVerified {
		t.Fatal("expected verified user")
	}
	if pub.verified != 1 {
		t.Fatalf("expected user.verified event, got %d", pub.verified)
	}

	err = uc.SendVerificationEmail(context.Background(), usecase.SendVerificationEmailInput{
		UserID: reg.User.ID,
	})
	if !errors.Is(err, usecase.ErrAlreadyVerified) {
		t.Fatalf("expected ErrAlreadyVerified, got %v", err)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepo()
	mailer := &mockMailer{}
	uc := newTestUseCase(t, repo, newMockJWT(), mailer, nil)

	_, err := uc.Register(context.Background(), usecase.RegisterInput{
		Username: "erin",
		Email:    "erin@example.com",
		Password: "oldpassword",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := uc.ForgotPassword(context.Background(), usecase.ForgotPasswordInput{
		Email: "erin@example.com",
	}); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}

	token := extractTokenParam(mailer.lastResetURL)
	if err := uc.ResetPassword(context.Background(), usecase.ResetPasswordInput{
		Token:       token,
		NewPassword: "newpassword123",
	}); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	_, err = uc.Login(context.Background(), usecase.LoginInput{
		Email:    "erin@example.com",
		Password: "newpassword123",
	})
	if err != nil {
		t.Fatalf("Login with new password: %v", err)
	}

	err = uc.ResetPassword(context.Background(), usecase.ResetPasswordInput{
		Token:       token,
		NewPassword: "another",
	})
	if !errors.Is(err, usecase.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for reused token, got %v", err)
	}
}

func TestForgotPasswordUnknownEmailIsSilent(t *testing.T) {
	t.Parallel()

	uc := newTestUseCase(t, newMockUserRepo(), newMockJWT(), &mockMailer{}, nil)
	if err := uc.ForgotPassword(context.Background(), usecase.ForgotPasswordInput{
		Email: "unknown@example.com",
	}); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
}

func extractTokenParam(url string) string {
	const key = "token="
	idx := 0
	for i := 0; i < len(url); i++ {
		if i+len(key) <= len(url) && url[i:i+len(key)] == key {
			idx = i + len(key)
			break
		}
	}
	return url[idx:]
}
