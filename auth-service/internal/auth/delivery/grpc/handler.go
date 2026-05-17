package grpc

import (
	"context"
	"errors"
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
		FirstName: req.GetFirstName(),
		LastName:  req.GetLastName(),
		Email:     req.GetEmail(),
		Password:  req.GetPassword(),
	})
	if err != nil {
		return nil, mapError(err)
	}

	// Send verification email after successful registration (best-effort).
	if sendErr := h.uc.SendVerificationEmail(ctx, usecase.SendVerificationEmailInput{
		Email: req.GetEmail(),
	}); sendErr != nil && !errors.Is(sendErr, usecase.ErrAlreadyVerified) {
		return nil, mapError(sendErr)
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

func (h *AuthHandler) SendVerificationEmail(ctx context.Context, req *authv1.SendVerificationEmailRequest) (*authv1.SendVerificationEmailResponse, error) {
	in := usecase.SendVerificationEmailInput{Email: req.GetEmail()}

	// Authenticated caller: use token from metadata when email is omitted.
	if req.GetEmail() == "" {
		userID, err := h.userIDFromContext(ctx)
		if err != nil {
			return nil, err
		}
		in.UserID = userID
	}

	if err := h.uc.SendVerificationEmail(ctx, in); err != nil {
		if errors.Is(err, usecase.ErrAlreadyVerified) {
			return &authv1.SendVerificationEmailResponse{
				Sent:    false,
				Message: "email is already verified",
			}, nil
		}
		return nil, mapError(err)
	}

	return &authv1.SendVerificationEmailResponse{
		Sent:    true,
		Message: "verification email sent",
	}, nil
}

func (h *AuthHandler) VerifyEmail(ctx context.Context, req *authv1.VerifyEmailRequest) (*authv1.VerifyEmailResponse, error) {
	user, err := h.uc.VerifyEmail(ctx, usecase.VerifyEmailInput{Token: req.GetToken()})
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.VerifyEmailResponse{
		Verified: true,
		User:     toProtoUser(user),
	}, nil
}

func (h *AuthHandler) ForgotPassword(ctx context.Context, req *authv1.ForgotPasswordRequest) (*authv1.ForgotPasswordResponse, error) {
	if err := h.uc.ForgotPassword(ctx, usecase.ForgotPasswordInput{Email: req.GetEmail()}); err != nil {
		return nil, mapError(err)
	}
	return &authv1.ForgotPasswordResponse{
		Accepted: true,
		Message:  "if the email exists, a reset link has been sent",
	}, nil
}

func (h *AuthHandler) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	if err := h.uc.ResetPassword(ctx, usecase.ResetPasswordInput{
		Token:       req.GetToken(),
		NewPassword: req.GetNewPassword(),
	}); err != nil {
		return nil, mapError(err)
	}
	return &authv1.ResetPasswordResponse{Success: true}, nil
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
