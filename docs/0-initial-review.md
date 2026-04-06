# Initial Codebase Review - Entain BE Technical Test

**Review Date:** 2026-04-06
**Last Updated:** 2026-04-06
**Scope:** Full codebase review of api/ and racing/ services

---

## Review Summary

This document has been updated to reflect the current state of the codebase. Several issues have been resolved, while others remain open and should be addressed before production deployment.

## Executive Summary

This document provides a comprehensive review of the Entain backend technical test codebase. The project demonstrates a microservices architecture with a REST API gateway (`api`) forwarding requests to a gRPC service (`racing`) that manages horse racing data. Overall, the codebase shows good architectural patterns (repository pattern, separation of concerns) but has several critical issues that need addressing before production use, particularly around resource management, error handling, and testing.

**Status:** Several critical issues have been resolved since the initial review. See the ✅ Resolved Issues section below for details.

## 🔴 Critical Issues

| File | Line | Issue | Severity | Suggestion | Status |
|------|------|-------|----------|------------|--------|
| `racing/go.mod` | - | Unreachable dependency `syreclabs.com/go/faker` | Build Failure | Add `replace syreclabs.com/go/faker => github.com/dmgk/faker v1.2.3` to redirect to GitHub source | ✅ **RESOLVED** |
| `racing/main.go` | 30-32 | Database connection never closed | Resource Leak | Add `defer racingDB.Close()` after opening the connection | ✅ **RESOLVED** |
| `racing/main.go` | 44-47 | No graceful shutdown handling | Reliability | Implement signal handling for SIGINT/SIGTERM to gracefully close connections and shutdown server | ✅ **RESOLVED** - Added `grpcServer.GracefulStop()` |
| `racing/db/races.go` | 57-62 | Database rows not closed (`defer rows.Close()` missing) | Resource Leak | Add `defer rows.Close()` immediately after error check on `db.Query()` | ✅ **RESOLVED** - Added in current code |
| `racing/db/db.go` | 15-26 | Seeding errors are swallowed in loop | Bug | Check and handle errors inside the for loop; current code continues seeding even after errors occur | ❌ Open |
| `racing/db/races.go` | 74-77 | `scanRaces` returns `nil` for `sql.ErrNoRows` | Bug | Return empty slice `[]*racing.Race{}` instead of `nil` for consistent API behavior | ❌ Open |
| `api/main.go` | 28 | Uses `grpc.WithInsecure()` | Security | Document this is for development only; use TLS in production with `grpc.WithTransportCredentials()` | ⚠️ Accepted for dev |
| `racing/db/races.go` | 55-56 | SQL query building uses string concatenation | Security | While parameterized args mitigate injection, consider using a query builder library for maintainability | ❌ Open |

## 🟡 Major Concerns

| File | Line | Issue | Category | Suggestion | Status |
|------|------|-------|----------|------------|--------|
| `racing/main.go` | 30 | Hardcoded database path `./db/racing.db` | Configuration | Add `-db-path` flag to configure database location | ❌ Open |
| `racing/main.go` | 23 | Hardcoded gRPC endpoint (ignores flag variable) | Configuration | Use `*grpcEndpoint` flag variable instead of hardcoded `:9000` | ✅ **RESOLVED** - Now uses `*grpcEndpoint` |
| `api/main.go` | 14-15 | Hardcoded endpoint addresses | Configuration | Use environment variables for `api-endpoint` and `grpc-endpoint` | ❌ Open |
| `racing/service/racing.go` | 24-27 | No input validation on filter parameters | Validation | Add validation for filter values (e.g., max meeting IDs, valid ranges) | ❌ Open |
| `racing/db/races.go` | 45-58 | No pagination support | Performance | Add `limit` and `offset` parameters to prevent large result sets | ❌ Open |
| `racing/db/races.go` | 45-58 | No sorting/ordering support | Functionality | Add `order_by` and `sort_direction` parameters to filter | ❌ Open |
| `racing/db/races.go` | 22 | Context not propagated to data layer | Best Practice | Pass context to `List()` method and use `QueryContext` instead of `Query` | ❌ Open |
| `racing/db/db.go` | 9-27 | No error handling during seeding | Reliability | Wrap seeding in transaction; rollback on error | ❌ Open |
| `api/main.go`, `racing/main.go` | - | No health check endpoints | Observability | Add `/health` or `/ready` endpoints for monitoring | ❌ Open |
| `api/main.go`, `racing/main.go` | - | No structured logging | Observability | Replace `log` package with `zap` or `logrus` for structured logging | ❌ Open |
| `**/*.go` | - | No test files present | Testing | Add comprehensive unit and integration tests (see testing recommendations) | ✅ **RESOLVED** - Tests added (`races_test.go`, `races_integration_test.go`) |
| `racing/db/races.go` | 61-62 | `strings.Repeat` for IN clause can be optimized | Performance | Consider using `?` placeholder generation or named parameters | ❌ Open |
| `racing/db/races.go` | 36-42 | `sync.Once` error handling not retried | Reliability | Document that seeding failures won't retry on subsequent calls | ❌ Open |
| `api/`, `racing/` | - | Generated proto files may be out of sync | Build | Pin tool versions in tools.go and add proto generation check to CI/CD | ❌ Open |

## 🟢 Minor Suggestions

- **File**: `racing/db/queries.go` - Consider using constants for column names to avoid typos across queries | ❌ Open
- **File**: `racing/service/racing.go` - Add comments to exported types and functions following Go conventions | ❌ Open
- **File**: `racing/proto/racing/racing.proto` - Consider adding `option go_package` with full import path instead of relative `/racing` | ❌ Open
- **File**: `api/tools.go`, `racing/tools.go` - Pin tool versions to specific commits/tags for reproducible builds | ❌ Open
- **File**: `racing/db/db.go` - Use `sql.Named()` for prepared statement parameters for better readability | ❌ Open
- **File**: `racing/main.go` - Consider connection pooling configuration for SQLite (e.g., `SetMaxOpenConns`, `SetMaxIdleConns`) | ❌ Open
- **File**: `README.md` - Add troubleshooting section for common development issues | ❌ Open
- **File**: `racing/db/races.go` - Use consistent receiver variable names (`r` instead of `m` in `scanRaces`) | ❌ Open
- **File**: `racing/db/races.go` - Add comment documenting SQL injection safety for string concatenation pattern | ❌ Open
- **File**: `racing/db/races.go` - Document boolean storage as INTEGER in SQLite (working as intended) | ❌ Open
- **File**: `Project root` - Add build scripts or Makefile with pre-commit targets (lint, test, generate, build) | ❌ Open
- **File**: `Project root` - Add `go.work` workspace file (recommended for multi-module workspaces to ensure consistent development experience) | ✅ **RESOLVED** - `go.work` added

## ✅ Positive Observations

- **Repository Pattern**: Clean separation between service logic and data access makes the codebase maintainable and testable
- **Proto-First Design**: Well-structured protobuf definitions with proper comments and field documentation
- **grpc-gateway Integration**: Seamless REST-to-gRPC translation reduces boilerplate code
- **Module Separation**: API and racing services are in separate Go modules, enabling independent versioning
- **Tools Pattern**: Using `tools.go` file to track build tool dependencies is a Go best practice
- **Interface Definition**: Service layer defines clear interfaces (`Racing`, `RacesRepo`) for easy mocking
- **Query Organization**: Centralized query storage in `queries.go` makes SQL management easier
- **Idempotent Seeding**: Using `INSERT OR IGNORE` prevents duplicate data on restart
- **sync.Once for Init**: Thread-safe initialization of the database repository
- **Google API Annotations**: Following Google API design patterns for HTTP mappings
- **Code Structure**: Consistent directory structure across services (`proto/`, `service/`, `db/`)
- **Dependency Management**: Using Go modules with explicit version pinning in `go.mod`
- **CI/CD Configuration**: `.gitlab-ci.yml` provides automated build pipeline
- **Testing**: Unit and integration tests have been added (`races_test.go`, `races_integration_test.go`)
- **Workspace**: `go.work` file added for multi-module workspace support
- **Resource Management**: Database rows are now properly closed with `defer rows.Close()`

## Testing Strategy Recommendations

Based on the testing guidelines in `CLAUDE.md`:

1. **In-Memory Database**: Use `:memory:` SQLite for test isolation
2. **Explicit Seeding**: Each test controls its own data via `seedData` field
3. **Cleanup**: Use `t.Cleanup()` for teardown
4. **Table-Driven Tests**: All test cases in one place with clear pass/fail per case
5. **Helper Functions**: Use `t.Helper()` for better stack traces
6. **End-to-End Flow**: Test through service layer, not just repository

### Suggested Test Structure

```
racing/
├── service/
│   ├── racing.go
│   └── racing_test.go    # Service layer tests
└── db/
    ├── races.go
    ├── races_test.go     # Repository tests with in-memory DB
    └── queries_test.go   # Query validation tests
```

## Pending Tasks Summary

Per `CLAUDE.md`, the following tasks are pending:

| Task | Priority | Estimated Effort | Files to Modify |
|------|----------|------------------|-----------------|
| 1. Visibility Filter | High | Low | `racing.proto`, `races.go` |
| 2. Ordering by Time | High | Medium | `racing.proto`, `races.go`, `queries.go` |
| 3. Status Field | Medium | Medium | `racing.proto`, `races.go` or `racing.go` |
| 4. GetRace RPC | High | Medium | `racing.proto`, `races.go`, `racing.go`, `queries.go` |
| 5. Sports Service | Medium | High | New `sports/` directory, `api/main.go` |

## Build & Development Notes

### Prerequisites
```bash
# Install Go
brew install go

# Install protoc
brew install protobuf

# Install protoc plugins (run in api/ and racing/)
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

## Recommended Priority Order

1. ~~**Fix dependency resolution** (syreclabs.com/go/faker)~~ - ✅ **RESOLVED**
2. ~~**Fix resource leak** (missing `defer rows.Close()`)~~ - ✅ **RESOLVED**
3. ~~**Add test files**~~ - ✅ **RESOLVED**
4. ~~**Fix hardcoded gRPC endpoint**~~ - ✅ **RESOLVED**
5. ~~**Add Go workspace**~~ - ✅ **RESOLVED**
6. ~~**Fix database connection leak** (missing `defer racingDB.Close()`)~~ - ✅ **RESOLVED**
7. ~~**Add graceful shutdown handling**~~ - ✅ **RESOLVED**
8. **Fix seeding error handling** (errors swallowed in loop) - 🟡 **MEDIUM PRIORITY**
9. **Fix scanRaces nil return** (return empty slice instead of nil) - 🟡 **MEDIUM PRIORITY**
10. **Add context support** (QueryContext) - 🟡 **MEDIUM PRIORITY**
11. **Fix hardcoded DB path** - 🟡 **MEDIUM PRIORITY**
12. **Address remaining low-severity issues** (receiver names, sync.Once documentation, proto generation)
13. **Implement pending technical test tasks (1-5)** - Core requirements

---

## Code Quality Assessment

### Strengths

- ✅ Clean separation of concerns (API gateway → gRPC → service → repository)
- ✅ Good use of protobuf/grpc-gateway pattern
- ✅ Repository pattern implemented correctly
- ✅ CI/CD configuration in place (`.gitlab-ci.yml`)
- ✅ Good documentation in `README.md`
- ✅ Unit and integration tests added
- ✅ Dependency resolution fixed
- ✅ Go workspace configured (`go.work`)
- ✅ Database rows properly closed (resource leak fixed)
- ✅ gRPC endpoint flag properly used

### Areas for Improvement

- ✅ Unit tests added (critical gap addressed)
- ✅ Database rows closed (resource leak fixed)
- ✅ Database connection closed (resource leak fixed)
- ❌ Missing context propagation
- ❌ Hardcoded configuration values (DB path)
- ✅ Graceful shutdown handling added
- ❌ Seeding error handling (errors swallowed in loop)
- ✅ Dependency resolution fixed
- ✅ gRPC endpoint flag fixed

---

## Conclusion

The codebase demonstrates a solid understanding of gRPC, protobuf, and Go microservice architecture. The repository pattern and clear separation of concerns are excellent starting points. **Several critical issues have been resolved** since the initial review, but some important issues remain:

### ✅ Resolved Since Initial Review

- **Dependency resolution** - `replace` directive added for `syreclabs.com/go/faker`
- **Test coverage** - Unit and integration tests added (`races_test.go`, `races_integration_test.go`)
- **Go workspace** - `go.work` file added for multi-module development
- **gRPC endpoint flag** - Now properly uses the `*grpcEndpoint` flag variable
- **Database rows closed** - `defer rows.Close()` added in `racing/db/races.go`

### ❌ Still Unresolved

- Seeding errors swallowed in loop (`racing/db/db.go`)
- `scanRaces` returns `nil` for `sql.ErrNoRows` instead of empty slice
- Missing context propagation to data layer
- Hardcoded database path
- No health check endpoints
- No structured logging

These should be addressed before implementing the pending technical test tasks to ensure a stable foundation. The pending technical test tasks provide a good opportunity to demonstrate API design skills while implementing the fixes above. Each task should be tackled as a separate PR/MR with appropriate tests.

**Overall Assessment:** The codebase is **functional for development** but still requires **stability improvements** before production deployment. Priority should be given to fixing the remaining resource leaks (database connection) and adding graceful shutdown handling.

**Next Steps:**
1. ~~Fix dependency resolution with replace directive~~ - ✅ **DONE**
2. ~~Fix database rows resource leak~~ - ✅ **DONE**
3. ~~Add comprehensive test coverage~~ - ✅ **DONE**
4. ~~Fix hardcoded gRPC endpoint~~ - ✅ **DONE**
5. ~~Add Go workspace~~ - ✅ **DONE**
6. ~~Fix database connection leak~~ - ✅ **DONE**
7. ~~Add graceful shutdown handling~~ - ✅ **DONE**
8. **Fix seeding error handling** - Check errors in seed loop, don't swallow them
9. **Fix scanRaces nil return** - Return empty slice for consistent API behavior
10. **Add context propagation** - Use `QueryContext` instead of `Query`
11. **Fix hardcoded database path** - Add `-db-path` flag
12. **Implement pending tasks** one at a time as separate PRs/MRs
13. **Add monitoring and observability** (health checks, structured logging)
14. **Consider containerization** (Docker) for easier deployment
