package grpc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	grpchandler "github.com/madiyar/final-project/auth-service/internal/auth/delivery/grpc"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	authv1 "github.com/madiyar/final-project/auth-service/internal/delivery/grpc/gen/auth/v1"
)

type mockUseCase struct {
	register func(context.Context, usecase.RegisterInput) (*usecase.AuthResult, error)
	login    func(context.Context, usecase.LoginInput) (*usecase.AuthResult, error)
	profile  func(context.Context, uuid.UUID) (*entity.User, error)
}

func (m *mockUseCase) Register(ctx context.Context, in usecase.RegisterInput) (*usecase.AuthResult, error) {
	return m.register(ctx, in)
}

func (m *mockUseCase) Login(ctx context.Context, in usecase.LoginInput) (*usecase.AuthResult, error) {
	return m.login(ctx, in)
}

func (m *mockUseCase) Logout(context.Context, usecase.LogoutInput) error { return nil }

func (m *mockUseCase) RefreshToken(context.Context, usecase.RefreshTokenInput) (*usecase.AuthResult, error) {
	return nil, nil
}

func (m *mockUseCase) GetProfile(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return m.profile(ctx, id)
}

func (m *mockUseCase) SendVerificationEmail(context.Context, usecase.SendVerificationEmailInput) error {
	return nil
}

func (m *mockUseCase) VerifyEmail(context.Context, usecase.VerifyEmailInput) (*entity.User, error) {
	return nil, nil
}

func (m *mockUseCase) ForgotPassword(context.Context, usecase.ForgotPasswordInput) error {
	return nil
}

func (m *mockUseCase) ResetPassword(context.Context, usecase.ResetPasswordInput) error {
	return nil
}

type mockTokenValidator struct{}

func (mockTokenValidator) ValidateAccessToken(string) (*jwtsvc.Claims, error) {
	return nil, jwtsvc.ErrInvalidToken
}

func TestRegisterMapsAlreadyExists(t *testing.T) {
	t.Parallel()

	uc := &mockUseCase{
		register: func(context.Context, usecase.RegisterInput) (*usecase.AuthResult, error) {
			return nil, usecase.ErrUserAlreadyExists
		},
	}
	h := grpchandler.NewAuthHandler(uc, mockTokenValidator{})

	_, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email:     "a@example.com",
		Password:  "password123",
		FirstName: "Alice",
		LastName:  "Smith",
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestLoginMapsUnauthenticated(t *testing.T) {
	t.Parallel()

	uc := &mockUseCase{
		login: func(context.Context, usecase.LoginInput) (*usecase.AuthResult, error) {
			return nil, usecase.ErrInvalidCredentials
		},
	}
	h := grpchandler.NewAuthHandler(uc, mockTokenValidator{})

	_, err := h.Login(context.Background(), &authv1.LoginRequest{
		Email:    "a@example.com",
		Password: "wrong",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestGetProfileRequiresAuthMetadata(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	uc := &mockUseCase{
		profile: func(_ context.Context, id uuid.UUID) (*entity.User, error) {
			if id != userID {
				return nil, usecase.ErrUnauthorized
			}
			return &entity.User{ID: id, Email: "a@example.com", Username: "alice"}, nil
		},
	}

	h := grpchandler.NewAuthHandler(uc, &validTokenValidator{userID: userID})

	_, err := h.GetProfile(context.Background(), &authv1.GetProfileRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without metadata, got %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer valid-token",
	))
	resp, err := h.GetProfile(ctx, &authv1.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if resp.GetUser().GetEmail() != "a@example.com" {
		t.Fatalf("unexpected user: %+v", resp.GetUser())
	}
}

type validTokenValidator struct {
	userID uuid.UUID
}

func (v *validTokenValidator) ValidateAccessToken(string) (*jwtsvc.Claims, error) {
	return &jwtsvc.Claims{UserID: v.userID, Email: "a@example.com"}, nil
}

func TestUnimplementedEndpoint(t *testing.T) {
	t.Parallel()

	h := grpchandler.NewAuthHandler(&mockUseCase{}, mockTokenValidator{})
	_, err := h.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented, got %v", err)
	}
}
