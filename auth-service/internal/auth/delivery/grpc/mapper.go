package grpc

import (
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	jwtsvc "github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
	authv1 "github.com/madiyar/final-project/auth-service/internal/delivery/grpc/gen/auth/v1"
)

func buildUsername(firstName, lastName, email string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)

	var b strings.Builder
	b.WriteString(strings.ToLower(first))
	b.WriteString(strings.ToLower(last))
	username := b.String()

	if username != "" {
		return username
	}

	if at := strings.Index(email, "@"); at > 0 {
		return strings.ToLower(email[:at])
	}
	return strings.ToLower(email)
}

func toProtoUser(u *entity.User) *authv1.User {
	if u == nil {
		return nil
	}
	return &authv1.User{
		Id:            u.ID.String(),
		Email:         u.Email,
		FirstName:     u.Username,
		LastName:      "",
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
