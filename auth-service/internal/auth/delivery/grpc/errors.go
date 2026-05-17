package grpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	"github.com/madiyar/final-project/auth-service/internal/domain/repository"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrUserAlreadyExists),
		errors.Is(err, repository.ErrEmailExists),
		errors.Is(err, repository.ErrUsernameExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, usecase.ErrInvalidCredentials),
		errors.Is(err, usecase.ErrUnauthorized),
		errors.Is(err, usecase.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())

	case errors.Is(err, usecase.ErrAlreadyVerified):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, usecase.ErrEmailSendFailed):
		return status.Error(codes.Unavailable, err.Error())

	case errors.Is(err, repository.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())

	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}
