package middleware

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests processed, partitioned by method and status code.",
		},
		[]string{"grpc_method", "grpc_code"},
	)

	grpcRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Histogram of gRPC request latencies in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"grpc_method"},
	)
)

func init() {
	prometheus.MustRegister(grpcRequestsTotal, grpcRequestDurationSeconds)
}

// UnaryMetricsInterceptor records Prometheus metrics for unary gRPC calls.
func UnaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		method := info.FullMethod
		code := status.Code(err).String()

		grpcRequestsTotal.WithLabelValues(method, code).Inc()
		grpcRequestDurationSeconds.WithLabelValues(method).Observe(time.Since(start).Seconds())

		return resp, err
	}
}
