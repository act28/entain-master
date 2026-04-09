package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"git.neds.sh/matty/entain/sports/service"
	"google.golang.org/grpc"
)

var (
	grpcEndpoint = flag.String("grpc-endpoint", "localhost:9001", "gRPC server endpoint")
	dbPath       = flag.String("db-path", "./db/sports.db", "Path to SQLite database file")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("failed running grpc server: %s\n", err)
	}
}

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := &net.ListenConfig{}
	conn, err := lc.Listen(ctx, "tcp", *grpcEndpoint)
	if err != nil {
		return fmt.Errorf("net listener: %w", err)
	}

	sportsDB, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	defer func() { _ = sportsDB.Close() }()

	eventsRepo := db.NewEventsRepo(sportsDB)
	if err := eventsRepo.Init(context.Background()); err != nil {
		return fmt.Errorf("repo init: %w", err)
	}

	grpcServer := grpc.NewServer()

	sports.RegisterSportsServer(
		grpcServer,
		service.NewSportsService(
			eventsRepo,
		),
	)

	// Setup signal handling for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run server in goroutine.
	go func() {
		log.Printf("gRPC server listening on: %s\n", *grpcEndpoint)
		if err := grpcServer.Serve(conn); err != nil {
			log.Printf("server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal.
	<-sigChan
	log.Println("shutdown signal received, gracefully stopping server...")

	grpcServer.GracefulStop()

	return nil
}
