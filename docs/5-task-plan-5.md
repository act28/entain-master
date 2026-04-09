# Task 5: Sports Service Implementation Plan

## Overview

This task implements a new **Sports Service** microservice that manages sports events. The service will provide a `ListEvents` RPC method similar to the existing `ListRaces` in the racing service. This demonstrates the ability to create a complete microservice following the established patterns in the codebase.

The sports service will:

- Run as a separate gRPC service on port 9001
- Provide REST API access through the existing API gateway
- Follow the same repository pattern and architecture as the racing service
- Support filtering, sorting, and visibility controls

## Architecture

```text
entain/
├─ api/                          # REST Gateway (port 8000)
│  └─ proto/sports/              # Sports gateway proto with HTTP annotations
│
├─ racing/                       # Existing gRPC Service (port 9000)
│
└─ sports/                       # NEW: gRPC Service (port 9001)
   ├─ proto/sports/
   │  └─ sports.proto            # Core sports service definitions
   ├─ service/
   │  └─ sports.go              # Service implementation
   ├─ db/
   │  ├─ events.go              # Events repository
   │  ├─ queries.go             # SQL queries
   │  └─ sports.db              # SQLite database
   ├─ main.go                   # gRPC server
   ├─ go.mod
   └─ tools.go
```

## Changes Required

### 1. Create Sports Service Module Structure

**File Path:** `sports/go.mod`
**Change Type:** Add
**Purpose:** Define the new sports service module

```sports/go.mod
module git.neds.sh/matty/entain/sports

go 1.16

require (
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.3.0
	github.com/mattn/go-sqlite3 v1.14.6
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	google.golang.org/genproto v0.0.0-20210226172003-ab064af71705
	google.golang.org/grpc v1.36.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.25.1-0.20201208041424-160c7477e0e8
	syreclabs.com/go/faker v1.2.3
)

replace syreclabs.com/go/faker => github.com/dmgk/faker v1.2.3
```

### 2. Create Sports Proto Definition

**File Path:** `sports/proto/sports/sports.proto`
**Change Type:** Add
**Purpose:** Define the Sports service RPC methods and message types

```sports/proto/sports/sports.proto
syntax = "proto3";
package sports;

import "google/protobuf/timestamp.proto";

option go_package = "/sports";

service Sports {
  // ListEvents will return a collection of all sports events.
  rpc ListEvents(ListEventsRequest) returns (ListEventsResponse) {}
}

/* Requests/Responses */

message ListEventsRequest {
  ListEventsRequestFilter filter = 1;
}

// Response to ListEvents call.
message ListEventsResponse {
  repeated Event events = 1;
}

// Filter for listing events.
message ListEventsRequestFilter {
  repeated int64 sport_ids = 1;
  // Optional filter for visible events. If not set, returns both visible and invisible events.
  optional bool visible = 2;
  // Optional field to specify which field to sort by.
  // Supported values: "advertised_start_time", "name", "id", "sport_id"
  // If not set, defaults to "advertised_start_time".
  optional string sort_by = 3;
  // Optional field to specify sort direction.
  // If true, sorts in descending order (latest first).
  // If false or not set, sorts in ascending order (earliest first).
  optional bool descending = 4;
}

/* Resources */

// An event resource.
message Event {
  // Status represents the current state of an event.
  enum Status {
    // UNSPECIFIED indicates an unknown status.
    UNSPECIFIED = 0;
    // OPEN indicates the event has not yet started.
    OPEN = 1;
    // CLOSED indicates the event has started or finished.
    CLOSED = 2;
  }
  // ID represents a unique identifier for the event.
  int64 id = 1;
  // SportID represents a unique identifier for the sport type.
  int64 sport_id = 2;
  // SportTypeName is the human-readable name of the sport type (e.g., "Football").
  string sport_type_name = 3;
  // Name is the official name given to the event.
  string name = 4;
  // Visible represents whether or not the event is visible.
  bool visible = 5;
  // AdvertisedStartTime is the time the event is advertised to start.
  google.protobuf.Timestamp advertised_start_time = 6;
  // Status represents whether the event is open or closed for betting.
  // Computed based on advertised_start_time vs current time:
  // - OPEN: when current time is strictly before advertised_start_time
  // - CLOSED: when current time is at or after advertised_start_time
  Status status = 7;
}
```

### 3. Create Database Queries

**File Path:** `sports/db/queries.go`
**Change Type:** Add
**Purpose:** Define SQL queries for events repository

```sports/db/queries.go
package db

const eventsList = "list"

func getEventQueries() map[string]string {
	return map[string]string{
		eventsList: `
			SELECT
				e.id,
				e.sport_id,
				st.name as sport_type_name,
				e.name,
				e.visible,
				e.advertised_start_time
			FROM events e
			JOIN sport_types st ON e.sport_id = st.id
		`,
	}
}```

### 4. Create Events Repository

**File Path:** `sports/db/events.go`
**Change Type:** Add
**Purpose:** Implement repository pattern for events data access following racing service patterns

```sports/db/events.go
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"

	"git.neds.sh/matty/entain/sports/proto/sports"
)

// ErrEventNotFound is returned when an event is not found in the database.
// Use errors.Is(err, ErrEventNotFound) to check for this error.
var ErrEventNotFound = errors.New("event not found")

// EventsRepo provides repository access to events.
type EventsRepo interface {
	// Init will initialise our events repository.
	Init() error

	// List will return a list of events.
	// Accepts context for cancellation support.
	List(ctx context.Context, filter *sports.ListEventsRequestFilter) ([]*sports.Event, error)
}}

// MaxSportIDs is the maximum number of sport IDs allowed in a filter.
// Exported for use by the service layer.
const MaxSportIDs = 100

type eventsRepo struct {
	db        *sql.DB
	init      sync.Once
	initError error // Stores error from failed initialization for retry logic
}

// NewEventsRepo creates a new events repository.
func NewEventsRepo(db *sql.DB) EventsRepo {
	return &eventsRepo{db: db}
}

// Init prepares the events repository dummy data.
// This method is safe to call multiple times - it uses sync.Once to ensure
// seeding only happens once, but returns any error that occurred.
func (r *eventsRepo) Init() error {
	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy events.
		r.initError = r.seed()
	})

	return r.initError
}

func (r *eventsRepo) List(ctx context.Context, filter *sports.ListEventsRequestFilter) ([]*sports.Event, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getEventQueries()[eventsList]

	// Step 1: Apply WHERE clause (filtering)
	query, args = r.applyFilter(query, filter)

	// Step 2: Apply ORDER BY (sorting)
	query = r.applySorting(query, filter)

	// Use QueryContext for cancellation support
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanEvents(ctx, rows)
}

// buildInClause creates an IN clause with the specified number of placeholders.
// Returns a string like "(?,?,?)" for count=3.
func buildInClause(count int) string {
	if count <= 0 {
		return "()"
	}
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return "(" + strings.Join(placeholders, ",") + ")"
}

func (r *eventsRepo) applyFilter(query string, filter *sports.ListEventsRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.SportIds) > 0 {
		// Use cleaner helper instead of strings.Repeat
		clauses = append(clauses, fmt.Sprintf("e.sport_id IN %s", buildInClause(len(filter.SportIds))))
		for _, sportID := range filter.SportIds {
			args = append(args, sportID)
		}
	}

	// Add visible filter if specified
	if filter.Visible != nil {
		clauses = append(clauses, "e.visible = ?")
		args = append(args, *filter.Visible)
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

// applySorting adds ORDER BY clause based on filter parameters.
// Defaults to ordering by advertised_start_time ASC.
func (r *eventsRepo) applySorting(query string, filter *sports.ListEventsRequestFilter) string {
	// Determine sort field (default to advertised_start_time)
	sortBy := "advertised_start_time"
	if filter != nil && filter.SortBy != nil && *filter.SortBy != "" {
		sortBy = r.validateSortField(*filter.SortBy)
	}

	// Determine sort direction (default ASC, only ASC or DESC allowed)
	direction := "ASC"
	if filter != nil && filter.Descending != nil && *filter.Descending {
		direction = "DESC"
	}

	return query + fmt.Sprintf(" ORDER BY %s %s", sortBy, direction)
}

// validateSortField ensures the sort field is valid to prevent SQL injection.
func (r *eventsRepo) validateSortField(field string) string {
	switch field {
	case "advertised_start_time", "name", "id", "sport_id":
		return field
	}
	// Return safe default if invalid field provided
	return "advertised_start_time"
}

// computeEventStatus determines if an event is OPEN or CLOSED based on the
// advertised start time compared to the current time.
func computeEventStatus(advertisedStart time.Time) sports.Event_Status {
	if time.Now().UTC().Before(advertisedStart.UTC()) {
		return sports.Event_OPEN
	}
	return sports.Event_CLOSED
}

// defaultEventCapacity is the default pre-allocation size for events slice.
// Based on typical query result sizes - Go's append will grow as needed.
const defaultEventCapacity = 32

func (r *eventsRepo) scanEvents(
	ctx context.Context,
	rows *sql.Rows,
) ([]*sports.Event, error) {
	// Pre-allocate with reasonable capacity to minimize allocations.
	// Uses empty slice (not nil) for consistent API behavior.
	events := make([]*sports.Event, 0, defaultEventCapacity)

	for rows.Next() {
		// Check for context cancellation to allow early exit on long-running queries
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("scanEvents cancelled: %w", ctx.Err())
		default:
		}
		var event sports.Event
		var advertisedStart time.Time

		if err := rows.Scan(&event.Id, &event.SportId, &event.SportTypeName, &event.Name, &event.Visible, &advertisedStart); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, fmt.Errorf("failed to convert timestamp: %w", err)
		}

		event.AdvertisedStartTime = ts

		// Compute status based on advertised_start_time vs current time
		event.Status = computeEventStatus(advertisedStart)

		events = append(events, &event)
	}

	// Check for errors that may have occurred during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event rows: %w", err)
	}

	return events, nil
}
```

### 5. Create Service Implementation

**File Path:** `sports/service/sports.go`
**Change Type:** Add
**Purpose:** Implement Sports service interface

```sports/service/sports.go
package service

import (
	"errors"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Sports interface {
	// ListEvents will return a collection of events.
	ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error)
}

// sportsService implements the Sports interface.
type sportsService struct {
	eventsRepo db.EventsRepo
}

// NewSportsService instantiates and returns a new sportsService.
func NewSportsService(eventsRepo db.EventsRepo) Sports {
	return &sportsService{eventsRepo}
}

func (s *sportsService) ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error) {
	// Validate filter parameters in service layer (business logic)
	if err := s.validateFilter(in.Filter); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid filter: %v", err)
	}

	// Pass context to repository for cancellation support
	events, err := s.eventsRepo.List(ctx, in.Filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list events: %v", err)
	}

	return &sports.ListEventsResponse{Events: events}, nil
}

// validateFilter validates the filter parameters and returns an error if invalid.
// Business logic validation belongs in the service layer, not the repository.
// Note: Does NOT validate sort_by to avoid schema exposure - invalid fields
// are silently handled by the repository's applySorting which falls back to a safe default.
func (s *sportsService) validateFilter(filter *sports.ListEventsRequestFilter) error {
	if filter == nil {
		return nil
	}

	// Validate max number of sport_ids to prevent excessive query complexity (DoS protection)
	if len(filter.SportIds) > db.MaxSportIDs {
		return fmt.Errorf("too many sport_ids: maximum is %d, got %d", db.MaxSportIDs, len(filter.SportIds))
	}

	// Note: sort_by is NOT validated here to avoid schema exposure.
	// Invalid sort fields are silently handled by the repository which falls back
	// to a safe default (advertised_start_time).

	return nil
}
```

 Point

**File Path:** `sports/main.go`
**Change Type:** Add
**Purpose:** gRPC server entry point for sports service

```sports/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"git.neds.sh/matty/entain/sports/service"
	"google.golang.org/grpc"
)

var (
	grpcEndpoint = flag.String("grpc-endpoint", "localhost:9100", "gRPC server endpoint")
	dbPath       = flag.String("db-path", "./db/sports.db", "Database file path")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("failed running grpc server: %s\n", err)
	}
}

func run() error {
	conn, err := net.Listen("tcp", *grpcEndpoint)
	if err != nil {
		return err
	}

	sportsDB, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := sportsDB.Close(); err != nil {
			log.Printf("error closing database: %v\n", err)
		}
	}()

	// Configure connection pool
	sportsDB.SetMaxOpenConns(1) // SQLite supports only one writer at a time
	sportsDB.SetMaxIdleConns(1)
	sportsDB.SetConnMaxLifetime(time.Hour)

	// Verify database connection
	if err := sportsDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	eventsRepo := db.NewEventsRepo(sportsDB)
	if err := eventsRepo.Init(); err != nil {
		return err
	}

	grpcServer := grpc.NewServer()

	sports.RegisterSportsServer(
		grpcServer,
		service.NewSportsService(
			eventsRepo,
		),
	)

	log.Printf("gRPC server listening on: %s\n", *grpcEndpoint)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down gRPC server...")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(conn); err != nil {
		return err
	}

	return nil
}
```

### 7. Create Tools File for Code Generation

**File Path:** `sports/tools.go`
**Change Type:** Add
**Purpose:** Track tool dependencies for code generation

```sports/tools.go
//go:build tools
// +build tools

package tools

// What is this file? https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "google.golang.org/genproto/googleapis/api"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
```

### 8. Create API Gateway Proto

**File Path:** `api/proto/sports/sports.proto`
**Change Type:** Add
**Purpose:** Gateway proto with HTTP annotations for REST API access

```api/proto/sports/sports.proto
syntax = "proto3";
package sports;

import "google/api/annotations.proto";
import "google/protobuf/timestamp.proto";

option go_package = "/sports";

service Sports {
  // ListEvents returns a list of all sports events.
  rpc ListEvents(ListEventsRequest) returns (ListEventsResponse) {
    option (google.api.http) = {
      post: "/v1/list-events"
      body: "*"
    };
  }
}
}
}

/* Requests/Responses */

// Request for ListEvents call.
message ListEventsRequest {
  ListEventsRequestFilter filter = 1;
}

// Response to ListEvents call.
message ListEventsResponse {
  repeated Event events = 1;
}

// Filter for listing events.
message ListEventsRequestFilter {
  repeated int64 sport_ids = 1;
  // Optional filter for visible events. If not set, returns both visible and invisible events.
  optional bool visible = 2;
  // Optional field to specify which field to sort by.
  // Supported values: "advertised_start_time", "name", "id", "sport_id"
  // If not set, defaults to "advertised_start_time".
  optional string sort_by = 3;
  // Optional field to specify sort direction.
  // If true, sorts in descending order (latest first).
  // If false or not set, sorts in ascending order (earliest first).
  optional bool descending = 4;
}

/* Resources */

// An event resource.
message Event {
  // Status represents the current state of an event.
  enum Status {
    // UNSPECIFIED indicates an unknown status.
    UNSPECIFIED = 0;
    // OPEN indicates the event has not yet started.
    OPEN = 1;
    // CLOSED indicates the event has started or finished.
    CLOSED = 2;
  }
  // ID represents a unique identifier for the event.
  int64 id = 1;
  // SportID represents a unique identifier for the sport type.
  int64 sport_id = 2;
  // SportTypeName is the human-readable name of the sport type (e.g., "Football").
  string sport_type_name = 3;
  // Name is the official name given to the event.
  string name = 4;
  // Visible represents whether or not the event is visible.
  bool visible = 5;
  // AdvertisedStartTime is the time the event is advertised to start.
  google.protobuf.Timestamp advertised_start_time = 6;
  // Status represents whether the event is open or closed for betting.
  Status status = 7;
}
```

### 9. Update API Gateway Main

**File Path:** `api/main.go`
**Change Type:** Modify
**Purpose:** Register sports service handler in the API gateway

Before:

```api/main.go
func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	if err := racing.RegisterRacingHandlerFromEndpoint(
		ctx,
		mux,
		*grpcEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return err
	}

	log.Printf("API server listening on: %s\n", *apiEndpoint)

	return http.ListenAndServe(*apiEndpoint, mux)
}
```

After:

```api/main.go
func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	
	// Register racing service handler
	// Register health check endpoint
	// Health check endpoint
	// TODO: Implement downstream service health checks using gRPC health protocol
	// Currently returns static OK, but should verify racing and sports services are reachable
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Future enhancement: Check gRPC service health before returning OK
		// - Create gRPC connections to racingEndpoint and sportsEndpoint
		// - Use grpc_health_v1.HealthClient to check service status
		// - Return 503 Service Unavailable if any downstream service is unhealthy
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","services":{"racing":"unknown","sports":"unknown"}}`))
	})

	if err := racing.RegisterRacingHandlerFromEndpoint(
		ctx,
		mux,
		*racingEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return err
	}

	// Register sports service handler
	if err := sports.RegisterSportsHandlerFromEndpoint(
		ctx,
		mux,
		*sportsEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return err
	}

	log.Printf("API server listening on: %s\n", *apiEndpoint)

	return http.ListenAndServe(*apiEndpoint, mux)
}
```

Also update the flags:

```api/main.go
var (
	apiEndpoint     = flag.String("api-endpoint", "localhost:8000", "API endpoint")
	racingEndpoint  = flag.String("racing-endpoint", "localhost:9000", "Racing gRPC server endpoint")
	sportsEndpoint  = flag.String("sports-endpoint", "localhost:9001", "Sports gRPC server endpoint")
)
```

### 10. Update Workspace Configuration

**File Path:** `go.work`
**Change Type:** Modify
**Purpose:** Add sports module to workspace

Before:

```go.work
go 1.25

use (
	./api
	./racing
)
```

After:

```go.work
go 1.25

use (
	./api
	./racing
	./sports
)
```

### 11. Create Database Seeding

**File Path:** `sports/db/seed.go`
**Change Type:** Add
**Purpose:** Seed dummy data for events (similar to racing service)

```sports/db/seed.go
package db

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"syreclabs.com/go/faker"
)

func (r *eventsRepo) seed() error {
	// Use a transaction for atomicity - all or nothing
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Rollback on error, otherwise commit at end
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("error rolling back transaction: %v", rbErr)
			}
		}
	}()

	// Create sport_types table first (for referential integrity)
	_, err = tx.Exec(`DROP TABLE IF EXISTS events`)
	if err != nil {
		return fmt.Errorf("failed to drop events table: %w", err)
	}
	_, err = tx.Exec(`DROP TABLE IF EXISTS sport_types`)
	if err != nil {
		return fmt.Errorf("failed to drop sport_types table: %w", err)
	}
	_, err = tx.Exec(`CREATE TABLE sport_types (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	if err != nil {
		return fmt.Errorf("failed to create sport_types table: %w", err)
	}

	// Seed sport_types (valid sport types for referential integrity)
	sportTypes := []struct {
		id   int64
		name string
	}{
		{1, "Football"},
		{2, "Basketball"},
		{3, "Tennis"},
		{4, "Cricket"},
		{5, "Rugby"},
	}

	for _, st := range sportTypes {
		_, err = tx.Exec(`INSERT INTO sport_types(id, name) VALUES (?, ?)`, st.id, st.name)
		if err != nil {
			return fmt.Errorf("failed to insert sport_type %d: %w", st.id, err)
		}
	}

	// Create events table with foreign key reference
	_, err = tx.Exec(`
		CREATE TABLE events (
			id INTEGER PRIMARY KEY,
			sport_id INTEGER NOT NULL,
			name TEXT,
			visible BOOLEAN,
			advertised_start_time TIMESTAMP,
			FOREIGN KEY (sport_id) REFERENCES sport_types(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	// Create dummy events - multiple events per sport type for realistic testing
	// Similar to racing service which creates 100 events
	const numEvents = 100
	for i := 1; i <= numEvents; i++ {
		// Cycle through sport types so multiple events share the same sport_id
		// This allows proper testing of sport_id filtering (e.g., "give me all football events")
		sportID := int64((i-1)%len(sportTypes)) + 1 // Cycles 1-5

		// Vary the advertised start times - some in past, some in future
		var startTime time.Time
		if i%3 == 0 {
			// Every 3rd event is in the past (CLOSED status)
			startTime = time.Now().Add(-time.Duration(i) * time.Hour)
		} else {
			// Other events are in the future (OPEN status)
			startTime = time.Now().Add(time.Duration(i) * time.Hour)
		}

		_, err = tx.Exec(
			`INSERT INTO events(id, sport_id, name, visible, advertised_start_time) VALUES (?, ?, ?, ?, ?)`,
			i,
			sportID,
			faker.Team().Name()+" vs "+faker.Team().Name(),
			i%2 == 0, // visible: alternating true/false
			startTime,
		)
		if err != nil {
			return fmt.Errorf("failed to insert event %d: %w", i, err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit seed transaction: %w", err)
	}

	return nil
}
```

### 12. Create Database Directory in Code

Instead of using `.gitkeep`, we should create the database directory programmatically to ensure it exists at runtime:

**Update in `sports/main.go`:**

Add directory creation before opening the database:

```go
import (
    "os"
    "path/filepath"
    // ... other imports
)

func run() error {
    // ... other setup ...

    // Create db directory if it doesn't exist (more robust than .gitkeep)
    dbDir := filepath.Dir(*dbPath)
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        return fmt.Errorf("failed to create database directory: %w", err)
    }

    sportsDB, err := sql.Open("sqlite3", *dbPath)
    if err != nil {
        return err
    }
    // ... rest of the code
}
```

**Why this is better than `.gitkeep`:**

- ✅ Works even if directory is deleted or doesn't exist in fresh clone
- ✅ Explicit and self-documenting
- ✅ Follows the principle of being defensive at runtime
- ✅ Matches production best practices (don't assume filesystem state)

## Implementation Steps

| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | Create sports service directory structure | `sports/proto/sports/`, `sports/service/`, `sports/db/`, `sports/tests/` | None |
| 2 | Create `go.mod` for sports module | `sports/go.mod` | None |
| 3 | Create sports proto definitions | `sports/proto/sports/sports.proto` | None |
| 4 | Create database queries file | `sports/db/queries.go` | None |
| 5 | Create events repository | `sports/db/events.go` | `sports/db/queries.go` |
| 6 | Create database seeding | `sports/db/seed.go` | `sports/db/events.go` |
| 7 | Create sports service implementation | `sports/service/sports.go` | `sports/db/events.go`, proto |
| 8 | Create main.go server entry point | `sports/main.go` | `sports/service/sports.go` |
| 9 | Create tools.go for codegen | `sports/tools.go` | None |
| 10 | Generate protobuf code for sports service | `sports/proto/sports/*.go` | Run `go generate ./...` in sports/ |
| 11 | Create API gateway proto | `api/proto/sports/sports.proto` | Copy of sports proto with annotations |
| 12 | Generate gateway code | `api/proto/sports/*.go` | Run `go generate ./...` in api/ |
| 13 | Update API gateway main.go | `api/main.go` | API proto generated |
| 14 | Update go.work | `go/work` | Add ./sports |
| 15 | Run go mod tidy in all modules | All go.mod files | Ensure dependencies resolved |
| 16 | Write unit tests for sports service | `sports/tests/events_test.go` | Service implementation complete |
| 17 | Write integration tests for API | `api/tests/sports_integration_test.go` | Gateway complete |
| 18 | Test full end-to-end flow | All services | Both services running |

## API/Interface Changes

### New REST Endpoints

**ListEvents**

- **Method:** POST
- **Path:** `/v1/list-events`
- **Request Body:**

  ```json
  {
    "filter": {
      "sport_ids": [1, 2],
      "visible": true,
      "sort_by": "advertised_start_time",
      "descending": false
    }
  }
  ```

- **Response:**

  ```json
  {
    "events": [
      {
        "id": 1,
        "sport_id": 1,
        "sport_type_name": "Football",
        "name": "Manchester United vs Liverpool",
        "visible": true,
        "advertised_start_time": "2024-01-15T14:00:00Z",
        "status": "OPEN"
      }
    ]
  }
  ```

### Example cURL Commands

```bash
# List all events
curl -X POST "http://localhost:8000/v1/list-events" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {}}'

# List events with filters
curl -X POST "http://localhost:8000/v1/list-events" \
  -H 'Content-Type: application/json' \
  -d '{
    "filter": {
      "sport_ids": [1, 2],
      "visible": true,
      "sort_by": "advertised_start_time",
      "descending": true
    }
  }'


```

## Testing Strategy

### Unit Tests

**File:** `sports/tests/helpers.go`

```sports/tests/helpers.go
package tests

import (
	"database/sql"
	"testing"
	"time"
)

// testEvent represents a test event entry for seeding the database
type testEvent struct {
	ID                  int64
	SportID             int64
	Name                string
	Visible             bool
	AdvertisedStartTime time.Time
}

// setupTestDB creates a test database with sport_types and events tables
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	// Create in-memory SQLite db for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create sport_types table first (for referential integrity)
	_, err = db.Exec(`
		CREATE TABLE sport_types (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create sport_types table: %v", err)
	}

	// Seed sport_types for tests
	_, err = db.Exec(`
		INSERT INTO sport_types (id, name) VALUES 
			(1, 'Football'),
			(2, 'Basketball'),
			(3, 'Tennis')
	`)
	if err != nil {
		t.Fatalf("failed to seed sport_types: %v", err)
	}

	// Create events table with foreign key
	_, err = db.Exec(`
		CREATE TABLE events (
			id INTEGER PRIMARY KEY,
			sport_id INTEGER NOT NULL,
			name TEXT,
			visible BOOLEAN,
			advertised_start_time TIMESTAMP,
			FOREIGN KEY (sport_id) REFERENCES sport_types(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create events table: %v", err)
	}

	return db, func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database: %v", err)
		}
	}
}

// seedTestData inserts test event data into the database
func seedTestData(t *testing.T, db *sql.DB, testEvents []testEvent) {
	t.Helper()

	if len(testEvents) == 0 {
		return
	}

	// Insert each event using parameterized query
	for _, e := range testEvents {
		_, err := db.Exec(
			`INSERT INTO events (id, sport_id, name, visible, advertised_start_time)
			 VALUES (?, ?, ?, ?, ?)`,
			e.ID, e.SportID, e.Name, e.Visible, e.AdvertisedStartTime,
		)
		if err != nil {
			t.Fatalf("failed to insert test event: %v", err)
		}
	}
}
```

**File:** `sports/tests/events_test.go`

Test cases to cover:

1. **ListEvents_Service**
   - Nil filter
   - Empty filter
   - Empty database
   - Filter by single sport_id
   - Filter by multiple sport_ids
   - Filter by visible=true
   - Filter by visible=false
   - Non-existent sport_ids
   - Combined filters (sport_ids + visible)

2. **ListEvents_Sorting**
   - Sort by advertised_start_time ASC (default)
   - Sort by advertised_start_time DESC
   - Sort by name ASC
   - Sort by name DESC
   - Sort by id ASC/DESC
   - Sort by sport_id ASC/DESC
   - Invalid sort_by field (should default)

3. **ListEvents_StatusValidation**
   - Verify status is OPEN when advertised_start_time is in the future
   - Verify status is CLOSED when advertised_start_time is now or past

### Integration Tests

**File:** `api/tests/sports_integration_test.go`

Test cases:

1. **HTTP Error Handling**
   - Invalid HTTP method
   - Invalid content type
   - Malformed JSON body
   - Empty body

2. **Response Field Validation**
   - All expected fields present
   - Data types correct
   - Status computed correctly

3. **End-to-End Flow**
   - Full request/response cycle through gateway
   - Filter application through HTTP
   - Sorting through HTTP parameters

### Test Data Schema

```go
type testEvent struct {
    ID                  int64
    SportID             int64
    Name                string
    Visible             bool
    AdvertisedStartTime time.Time
}
```

Sample test setup with seed data:

```go
// Sport types 1-3 are automatically seeded in setupTestDB()
// Available: 1=Football, 2=Basketball, 3=Tennis

defaultTestEvents := []testEvent{
    {ID: 1, SportID: 1, Name: "Football Match A", Visible: true, AdvertisedStartTime: now.Add(2 * time.Hour)},
    {ID: 2, SportID: 1, Name: "Football Match B", Visible: false, AdvertisedStartTime: now.Add(4 * time.Hour)},
    {ID: 3, SportID: 2, Name: "Basketball Game", Visible: true, AdvertisedStartTime: now.Add(-1 * time.Hour)}, // CLOSED
    {ID: 4, SportID: 3, Name: "Tennis Match", Visible: true, AdvertisedStartTime: now.Add(6 * time.Hour)},
}

// In test - sport_types already seeded by setupTestDB:
db, cleanup := setupTestDB(t)
t.Cleanup(cleanup)
seedTestData(t, db, defaultTestEvents)  // Only need to seed events
```

## Regeneration Steps

### Initial Setup (run in sports/ directory)

```bash
cd sports

# Download dependencies
go mod tidy

# Install protoc plugins
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
  google.golang.org/genproto/googleapis/api \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc \
  google.golang.org/protobuf/cmd/protoc-gen-go

# Generate protobuf code
go generate ./...
```

### Protoc Generation Commands

If `go generate` is not configured, run these manually:

**In sports/ directory:**

```bash
# Generate Go code from proto
protoc -I . --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/sports/sports.proto
```

**In api/ directory:**

```bash
# Generate gateway code
protoc -I . --grpc-gateway_out=. \
    --grpc-gateway_opt=paths=source_relative \
    --grpc-gateway_opt=generate_unbound_methods=true \
    proto/sports/sports.proto

# Generate Go code
protoc -I . --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/sports/sports.proto
```

### Verification Commands

```bash
# Verify all modules compile
cd entain-master
go build ./...

# Run tests
cd racing && go test ./...
cd ../sports && go test ./...
cd ../api && go test ./...
```

## Running the Services

### Terminal 1: Racing Service

```bash
cd racing
go build && ./racing
# or: go run main.go
```

### Terminal 2: Sports Service

```bash
cd sports
go build && ./sports
# or: go run main.go
```

### Terminal 3: API Gateway

```bash
cd api
go build && ./api
# or: go run main.go
```

## Potential Risks/Considerations

### Backward Compatibility

- **Risk:** New service adds additional ports and endpoints
- **Mitigation:** Services are independent; existing racing service continues to work
- **API Gateway:** Changes to main.go add new handler but don't modify existing racing endpoints

### Database Schema

- **Risk:** Events table needs to be created
- **Mitigation:** Seed function creates table if not exists, similar to racing service pattern
- **Migration:** No migration needed for SQLite in-memory/test databases
- **Production Consideration:** For production deployments with persistent SQLite/PostgreSQL/MySQL databases, implement a formal migration strategy:
  - Use a migration tool like [golang-migrate](https://github.com/golang-migrate/migrate) or [goose](https://github.com/pressly/goose)
  - Create versioned SQL migration files (e.g., `001_create_sport_types.sql`, `002_create_events.sql`)
  - Run migrations on service startup before handling requests
  - Separate schema migrations from data seeding (seed only for dev/test environments)
  - Consider read-replica support for zero-downtime migrations

### Port Conflicts

- **Risk:** Sports service uses port 9001 which might be in use
- **Mitigation:** Port is configurable via `--grpc-endpoint` flag
- **Documentation:** Clear documentation on port requirements

### Performance

- **Risk:** Sorting by non-indexed fields could be slow
- **Current:** SQLite with small dataset, no performance concerns expected
- **Future:** If scaling, consider adding indexes to frequently sorted/filtered columns

### Code Duplication

- **Risk:** Similar patterns between racing and sports services
- **Current:** Intentional duplication following DRY principle within service boundaries
- **Future:** Could extract common patterns into shared library if more services added

### Error Handling

- **Risk:** Inconsistent error handling between services
- **Mitigation:** Following same patterns as racing service (status codes, error messages)
- **Testing:** Comprehensive error case tests

### Timezone Handling

- **Risk:** Status computation depends on time comparison
- **Mitigation:** Using UTC consistently, same as racing service
- **Testing:** Tests should use fixed times to avoid flakiness

### Dependencies

- **Risk:** Module version mismatches
- **Mitigation:** Using same versions as racing service in go.mod
- **Workspace:** Go workspace ensures consistent dependency resolution

### Testing Isolation

- **Risk:** Tests sharing state
- **Mitigation:** Each test uses `:memory:` SQLite database
- **Cleanup:** Proper test cleanup with `t.Cleanup()`

## Checklist Before Marking Complete

- [ ] Sports service compiles and runs on port 9001
- [ ] API gateway registers sports handlers successfully
- [ ] ListEvents endpoint works with various filters
- [ ] Status field computed correctly (OPEN/CLOSED)
- [ ] Sorting works for all supported fields
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] cURL examples from documentation work correctly
- [ ] Code follows existing patterns and style
- [ ] Documentation is complete and accurate
