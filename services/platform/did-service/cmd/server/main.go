package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	srv := grpc.NewServer(
	// interceptors wired in by shared pkg/middleware
	)

	// health check — required by all services
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())

	// enable reflection for grpcurl / tooling
	reflection.Register(srv)

	log.Info("did-service starting", zap.String("port", port))

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatal("serve failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("graceful shutdown")
	srv.GracefulStop()
}
