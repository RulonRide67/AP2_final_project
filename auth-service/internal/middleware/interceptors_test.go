package middleware_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/madiyar/final-project/auth-service/internal/middleware"
)

func TestUnaryLoggingInterceptor(t *testing.T) {
	t.Parallel()

	log := zaptest.NewLogger(t)
	interceptor := middleware.UnaryLoggingInterceptor(log)

	called := false
	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{
		FullMethod: "/auth.v1.AuthService/Login",
	}, func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	if !called || err != nil || resp != "ok" {
		t.Fatalf("handler not invoked correctly: called=%v err=%v resp=%v", called, err, resp)
	}
}

func TestUnaryMetricsInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := middleware.UnaryMetricsInterceptor()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{
		FullMethod: "/auth.v1.AuthService/Register",
	}, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.AlreadyExists, "exists")
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("unexpected code: %v", status.Code(err))
	}
}

func TestUnaryLoggingInterceptorNilLogger(t *testing.T) {
	t.Parallel()

	interceptor := middleware.UnaryLoggingInterceptor(nil)
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) {
		return nil, errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
