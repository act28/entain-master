# CLAUDE.md - Entain BE Technical Test

## Project Overview

This is an Entain backend technical test project demonstrating gRPC/REST API development in Go. The project consists of two microservices:

1. **`api/`** - REST API gateway that forwards requests to gRPC services
2. **`racing/`** - gRPC service for managing horse racing data

## Architecture

```
entain/
├─ api/                          # REST Gateway (port 8000)
│  ├─ proto/
│  │  ├─ racing/racing.proto     # Proto with HTTP annotations
│  │  └─ google/api/             # Google API annotations
│  ├─ main.go                    # HTTP server with grpc-gateway
│  ├─ go.mod
│  └─ tools.go
│
├─ racing/                       # gRPC Service (port 9000)
│  ├─ proto/
│  │  └─ racing/racing.proto     # Core proto definitions
│  ├─ service/
│  │  └─ racing.go              # Service implementation
│  ├─ db/
│  │  ├─ races.go               # Repository pattern
│  │  ├─ queries.go             # SQL queries
│  │  └─ racing.db              # SQLite database
│  ├─ main.go                   # gRPC server
│  ├─ go.mod
│  └─ tools.go
```

## Tech Stack

- **Language**: Go 1.16+
- **RPC**: gRPC with protobuf
- **Gateway**: grpc-gateway v2 (REST ↔ gRPC translation)
- **Database**: SQLite3
- **Pattern**: Repository pattern for data access

## Key Files

### Proto Definitions
- `racing/proto/racing/racing.proto` - Core service definition
- `api/proto/racing/racing.proto` - Gateway proto with HTTP annotations

### Service Layer
- `racing/service/racing.go` - Implements `Racing` interface with `ListRaces()` method

### Data Layer
- `racing/db/races.go` - `RacesRepo` interface and implementation
- `racing/db/queries.go` - SQL query management

## Build & Run

### Prerequisites
```bash
# Install Go
brew install go

# Install protoc
brew install protobuf

# Install protoc plugins (run in each service directory)
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
  google.golang.org/genproto/googleapis/api \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc \
  google.golang.org/protobuf/cmd/protoc-gen-go
```

### Start Services
```bash
# Terminal 1: Start racing service (gRPC on :9000)
cd racing
go build && ./racing

# Terminal 2: Start API gateway (REST on :8000)
cd api
go build && ./api
```

### Test API
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {}}'
```

### Generate Protos
```bash
# Run in api/ or racing/ directories
go generate ./...
```

## Current API

### ListRaces
- **Endpoint**: `POST /v1/list-races`
- **Filter**: `meeting_ids` (repeated int64)
- **Response**: List of `Race` objects

### Race Schema
```protobuf
message Race {
  int64 id = 1;
  int64 meeting_id = 2;
  string name = 3;
  int64 number = 4;
  bool visible = 5;
  google.protobuf.Timestamp advertised_start_time = 6;
}
```

## Pending Tasks (Technical Test)

1. **Visibility Filter** - Add `visible` filter to `ListRacesRequestFilter`
2. **Ordering** - Order by `advertised_start_time` (add sort params)
3. **Status Field** - Add computed `status` (OPEN/CLOSED) based on time
4. **Get Race** - Implement `GetRace` RPC for single race by ID
5. **Sports Service** - Create separate `sports` service with `ListEvents`

## Development Guidelines

- Follow [Google API Design](https://cloud.google.com/apis/design)
- Follow [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- Each task should be a separate PR/MR
- Document and comment all code
- Use standard method naming (List, Get, Create, etc.)

## Testing Guidelines

1. **Use in-memory database**: Each test should use `:memory:` SQLite for isolation
2. **Explicit data seeding**: Each test should control its own data via `seedData` field
3. **No shared state**: Tests should use `t.Cleanup()` for teardown
4. **Table-driven tests**: All test cases in one place with clear pass/fail per case
5. **Helper functions**: `t.Helper()` used in helper functions for better stack traces
6. **End-to-end flow**: Tests should use the service or api entrypoint to test the end-to-end pipeline

## Database Schema

```sql
CREATE TABLE races (
  id INTEGER PRIMARY KEY,
  meeting_id INTEGER,
  name TEXT,
  number INTEGER,
  visible BOOLEAN,
  advertised_start_time TIMESTAMP
);
```

## Module Paths

- API: `git.neds.sh/matty/entain/api`
- Racing: `git.neds.sh/matty/entain/racing`
