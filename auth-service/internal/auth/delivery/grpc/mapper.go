package grpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	authv1 "github.com/madiyar/final-project/auth-service/internal/delivery/grpc/gen/auth/v1"
)

func toProtoUser(u *entity.User) *authv1.User {
	if u == nil {
		return nil
	}
	firstName := u.FirstName
	lastName := u.LastName
	// Fallback for accounts created before first_name/last_name columns existed.
	if firstName == "" && lastName == "" && u.Username != "" {
		firstName = u.Username
	}
	return &authv1.User{
		Id:            u.ID.String(),
		Email:         u.Email,
		FirstName:     firstName,
		LastName:      lastName,
		EmailVerified: u.IsVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
	}
}

func toProtoTokenPair(t *jwtsvc.TokenPair) *authv1.TokenPair {
	if t == nil {
		return nil
	}
	return &authv1.TokenPair{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    t.ExpiresIn,
		TokenType:    t.TokenType,
	}
}

func toRegisterResponse(res *usecase.AuthResult) *authv1.RegisterResponse {
	return &authv1.RegisterResponse{
		User:   toProtoUser(res.User),
		Tokens: toProtoTokenPair(res.Tokens),
	}
}

func toLoginResponse(res *usecase.AuthResult) *authv1.LoginResponse {
	return &authv1.LoginResponse{
		User:   toProtoUser(res.User),
		Tokens: toProtoTokenPair(res.Tokens),
	}
}

func toRefreshTokenResponse(res *usecase.AuthResult) *authv1.RefreshTokenResponse {
	return &authv1.RefreshTokenResponse{
		Tokens: toProtoTokenPair(res.Tokens),
	}
}
