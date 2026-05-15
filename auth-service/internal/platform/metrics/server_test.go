package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/madiyar/final-project/auth-service/internal/platform/metrics"
)

func TestNewServerAndShutdown(t *testing.T) {
	t.Parallel()

	srv := metrics.NewServer("127.0.0.1:0")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
