package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	grpchandler "github.com/madiyar/final-project/auth-service/internal/auth/delivery/grpc"
	"github.com/madiyar/final-project/auth-service/internal/auth/jwt"
	"github.com/madiyar/final-project/auth-service/internal/auth/repository/postgres"
	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
	authv1 "github.com/madiyar/final-project/auth-service/internal/delivery/grpc/gen/auth/v1"
	"github.com/madiyar/final-project/auth-service/internal/middleware"
	"github.com/madiyar/final-project/auth-service/internal/platform/config"
	"github.com/madiyar/final-project/auth-service/internal/platform/logger"
	metricssrv "github.com/madiyar/final-project/auth-service/internal/platform/metrics"
	"github.com/madiyar/final-project/auth-service/pkg/email"
)

const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := connectPostgres(ctx, cfg, log)
	if err != nil {
		return err
	}

	redisClient, err := connectRedis(ctx, cfg, log)
	if err != nil {
		pool.Close()
		return err
	}

	natsConn, err := connectNATS(cfg, log)
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return err
	}

	metricsServer := metricssrv.NewServer(cfg.MetricsAddr())
	go func() {
		log.Info("metrics server listening", zap.String("addr", cfg.MetricsAddr()))
		if err := metricsServer.Start(); err != nil {
			log.Error("metrics server stopped", zap.Error(err))
		}
	}()

	refreshStore := jwt.NewRedisRefreshStore(redisClient)
	jwtSvc, err := jwt.NewService(cfg.JWT, refreshStore)
	if err != nil {
		log.Error("init jwt service", zap.Error(err))
		return err
	}

	mailer, err := email.NewClient(cfg.SMTP)
	if err != nil {
		log.Error("init smtp client", zap.Error(err))
		return err
	}

	userRepo := postgres.NewUserRepository(pool)
	eventPublisher := usecase.NewNATSEventPublisher(natsConn, cfg.NATSSubjectPrefix)
	authUC := usecase.NewAuthUseCase(
		userRepo,
		jwtSvc,
		redisClient,
		usecase.BcryptHasher{},
		mailer,
		eventPublisher,
		cfg.UseCase,
	)

	authHandler := grpchandler.NewAuthHandler(authUC, jwtSvc)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryMetricsInterceptor(),
			middleware.UnaryLoggingInterceptor(log),
		),
	)
	authv1.RegisterAuthServiceServer(grpcServer, authHandler)

	lis, err := net.Listen("tcp", cfg.GRPCAddr())
	if err != nil {
		log.Error("listen gRPC", zap.Error(err))
		return err
	}

	go func() {
		log.Info("gRPC server listening",
			zap.String("addr", cfg.GRPCAddr()),
			zap.String("app", cfg.AppName),
			zap.String("env", cfg.AppEnv),
		)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server stopped", zap.Error(err))
		}
	}()

	waitForShutdown(log, grpcServer, metricsServer, pool, redisClient, natsConn)
	return nil
}

func connectPostgres(ctx context.Context, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = cfg.PostgresMaxConns
	poolCfg.MinConns = cfg.PostgresMinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	log.Info("connected to PostgreSQL")
	return pool, nil
}

func connectRedis(ctx context.Context, cfg *config.Config, log *zap.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	log.Info("connected to Redis", zap.String("addr", cfg.RedisAddr))
	return client, nil
}

func connectNATS(cfg *config.Config, log *zap.Logger) (*nats.Conn, error) {
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, err
	}
	log.Info("connected to NATS", zap.String("url", cfg.NATSURL))
	return nc, nil
}

func waitForShutdown(
	log *zap.Logger,
	grpcServer *grpc.Server,
	metricsServer *metricssrv.Server,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	natsConn *nats.Conn,
) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
		log.Warn("gRPC graceful stop timed out, forcing stop")
		grpcServer.Stop()
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics server shutdown", zap.Error(err))
	}

	pool.Close()
	if err := redisClient.Close(); err != nil {
		log.Error("redis close", zap.Error(err))
	}
	natsConn.Close()

	log.Info("shutdown complete")
}
