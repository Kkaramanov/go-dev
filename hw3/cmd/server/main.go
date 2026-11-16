package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	moviepb "github.com/Kkaramanov/go-dev/hw3/api"
	"github.com/Kkaramanov/go-dev/hw3/cmd/server/config"
	"github.com/Kkaramanov/go-dev/hw3/internal/adapter/stand"
	"github.com/Kkaramanov/go-dev/hw3/internal/handler/grpc"
	"github.com/Kkaramanov/go-dev/hw3/internal/handler/http"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	defer func() { _ = httpListener.Close() }()
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	defer func() { _ = grpcListener.Close() }()

	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	svc := usecase.New(stand.New(cfg.MovieBaseURL, client, 5*time.Second), 10)
	httpServer := &http.Server{
		Handler:           httpapi.New(svc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      usecase.RequestTimeout + 5*time.Second,
		IdleTimeout:       time.Minute,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	grpcServer := grpc.NewServer()
	moviepb.RegisterMovieServiceServer(grpcServer, grpcapi.New(svc))
	reflection.Register(grpcServer)

	errorsCh := make(chan error, 2)
	go func() { errorsCh <- httpServer.Serve(httpListener) }()
	go func() { errorsCh <- grpcServer.Serve(grpcListener) }()
	slog.Info("service started", "http", httpListener.Addr(), "grpc", grpcListener.Addr(), "stand", cfg.MovieBaseURL)

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errorsCh:
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP shutdown timed out", "error", err)
		_ = httpServer.Close()
	}

	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
		<-grpcStopped
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, grpc.ErrServerStopped) {
		return serveErr
	}
	return nil
}
