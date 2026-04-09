package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.neds.sh/matty/entain/api/proto/racing"
	"git.neds.sh/matty/entain/api/proto/sports"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultTimeout           = 10 * time.Second
	defaultIdleTimeout       = 30 * time.Second
)

var (
	apiEndpoint    = flag.String("api-endpoint", "localhost:8000", "API endpoint")
	grpcEndpoint   = flag.String("grpc-endpoint", "localhost:9000", "Racing gRPC server endpoint")
	sportsEndpoint = flag.String("sports-endpoint", "localhost:9001", "Sports gRPC server endpoint")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Printf("failed running api server: %s\n", err)
	}
}

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()

	// Register racing handlers
	if err := racing.RegisterRacingHandlerFromEndpoint(
		ctx,
		mux,
		*grpcEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return fmt.Errorf("registering racing handlers: %w", err)
	}

	// Register sports handlers
	if err := sports.RegisterSportsHandlerFromEndpoint(
		ctx,
		mux,
		*sportsEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return fmt.Errorf("registering sports handlers: %w", err)
	}

	log.Printf("API server listening on: %s\n", *apiEndpoint)

	server := &http.Server{
		Addr:              *apiEndpoint,
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultTimeout,
		WriteTimeout:      defaultTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run server in goroutine
	go func() {
		log.Printf("API server listening on: %s\n", *apiEndpoint)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("shutdown signal received, gracefully stopping server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), defaultIdleTimeout)
	defer shutdownCancel()

	// Gracefully shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Cancel gRPC handler context
	cancel()

	log.Println("server stopped gracefully")

	return nil
}
