package grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	authv1 "github.com/madiyar/final-project/auth-service/internal/delivery/grpc/gen/auth/v1"
)

// TokenValidator validates Bearer access tokens from gRPC metadata.
type TokenValidator interface {
	ValidateAccessToken(tokenString string) (*jwtsvc.Claims, error)
}

// AuthHandler implements auth.v1.AuthServiceServer.
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	uc    usecase.AuthUseCase
	token TokenValidator
}

// NewAuthHandler creates a gRPC handler for the auth service.
func NewAuthHandler(uc usecase.AuthUseCase, tokenValidator TokenValidator) *AuthHandler {
	return &AuthHandler{uc: uc, token: tokenValidator}
}

// Register creates a new user account.
func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	res, err := h.uc.Register(ctx, usecase.RegisterInput{
		Username: buildUsername(req.GetFirstName(), req.GetLastName(), req.GetEmail()),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toRegisterResponse(res), nil
}

// Login authenticates a user and returns tokens.
func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	res, err := h.uc.Login(ctx, usecase.LoginInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toLoginResponse(res), nil
}

// Logout revokes the refresh token.
func (h *AuthHandler) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := h.uc.Logout(ctx, usecase.LogoutInput{
		RefreshToken: req.GetRefreshToken(),
	}); err != nil {
		return nil, mapError(err)
	}
	return &authv1.LogoutResponse{Success: true}, nil
}

// RefreshToken rotates the refresh token and returns a new token pair.
func (h *AuthHandler) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	res, err := h.uc.RefreshToken(ctx, usecase.RefreshTokenInput{
		RefreshToken: req.GetRefreshToken(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toRefreshTokenResponse(res), nil
}

// GetProfile returns the authenticated user's profile.
func (h *AuthHandler) GetProfile(ctx context.Context, _ *authv1.GetProfileRequest) (*authv1.GetProfileResponse, error) {
	userID, err := h.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.uc.GetProfile(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	return &authv1.GetProfileResponse{User: toProtoUser(user)}, nil
}

func (h *AuthHandler) ValidateToken(context.Context, *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ValidateToken not implemented")
}

func (h *AuthHandler) UpdateProfile(context.Context, *authv1.UpdateProfileRequest) (*authv1.UpdateProfileResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method UpdateProfile not implemented")
}

func (h *AuthHandler) ChangePassword(context.Context, *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ChangePassword not implemented")
}

func (h *AuthHandler) SendVerificationEmail(context.Context, *authv1.SendVerificationEmailRequest) (*authv1.SendVerificationEmailResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method SendVerificationEmail not implemented")
}

func (h *AuthHandler) VerifyEmail(context.Context, *authv1.VerifyEmailRequest) (*authv1.VerifyEmailResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method VerifyEmail not implemented")
}

func (h *AuthHandler) ForgotPassword(context.Context, *authv1.ForgotPasswordRequest) (*authv1.ForgotPasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ForgotPassword not implemented")
}

func (h *AuthHandler) ResetPassword(context.Context, *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ResetPassword not implemented")
}

func (h *AuthHandler) userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	if h.token == nil {
		return uuid.Nil, status.Error(codes.Internal, "token validator not configured")
	}

	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	claims, err := h.token.ValidateAccessToken(token)
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
	}

	return claims.UserID, nil
}

func bearerTokenFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	raw := strings.TrimSpace(values[0])
	if len(raw) < 8 || !strings.EqualFold(raw[:7], "bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization scheme")
	}

	token := strings.TrimSpace(raw[7:])
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "empty bearer token")
	}
	return token, nil
}
