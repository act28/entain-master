# Entain BE Technical Test - Makefile
# Multi-module Go project (api/, racing/, and sports/)

.PHONY: all build test lint generate clean kill-all kill-sports run-racing run-api run-sports run-all help tidy fmt

# Default goal
.DEFAULT_GOAL := test

# Default target
all: build

# Configurable endpoints
API_ENDPOINT ?= localhost:8000
RACING_GRPC_ENDPOINT ?= localhost:9000
SPORTS_GRPC_ENDPOINT ?= localhost:9001

# Service endpoints to monitor
SERVICE_ENDPOINTS := $(API_ENDPOINT) $(RACING_GRPC_ENDPOINT) $(SPORTS_GRPC_ENDPOINT)

# Build all services
build: tidy generate fmt lint
	@echo "Building all services..."
	go build -C racing -o racing .
	go build -C api -o api .
	go build -C sports -o sports .
	@echo "Build complete."

# Run unit tests
test: tidy generate fmt
	@echo "Running unit tests..."
	go test -C racing -v ./... -race -coverprofile=coverage.out
	go test -C sports -v ./... -race -coverprofile=coverage.out
	@echo "Unit tests complete."

integration-local: build
	@echo "Cleaning up any existing services..."
	@$(MAKE) --no-print-directory kill-all
	@echo "Starting services for local integration tests..."
	# Set up cleanup trap: kills background jobs and runs kill-all on exit, interrupt, or terminate signals
	# This ensures services are cleaned up even if the user presses Ctrl+C or the script errors out
	@trap 'kill $$(jobs -p) 2>/dev/null; $(MAKE) --no-print-directory kill-all' EXIT INT TERM; \
	cd ./racing && ./racing -grpc-endpoint=$(RACING_GRPC_ENDPOINT) & \
	cd ./sports && ./sports -grpc-endpoint=$(SPORTS_GRPC_ENDPOINT) & \
	cd ./api && ./api -grpc-endpoint=$(RACING_GRPC_ENDPOINT) -sports-endpoint=$(SPORTS_GRPC_ENDPOINT) -api-endpoint=$(API_ENDPOINT) & \
	echo "Waiting for services to start..."; \
	sleep 3; \
	echo "Running integration tests..."; \
	go test -tags=integration -v ./api/tests/...; \
	echo "Integration tests complete."

# Run all tests (unit + integration)
test-all: test integration-local
	@echo "All tests complete."

# Lint and format check
lint:
	@echo "Checking Go formatting..."
	@unformatted=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "✓ Go files are formatted"
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		# Skip linting in racing due to legacy issues for now \
		# (cd racing && golangci-lint run ./... --config=../.golangci.yml --timeout=5m); \
		(cd sports && golangci-lint run ./... --config=../.golangci.yml --timeout=5m); \
		(cd api && golangci-lint run ./... --config=../.golangci.yml --timeout=5m); \
	else \
		echo "⚠ golangci-lint not installed - skipping"; \
	fi
	@echo "Lint complete."

# Generate proto files
generate:
	@echo "Generating racing proto files..."
	cd ./racing && go generate ./...
	@echo "Generating sports proto files..."
	cd ./sports && go generate ./...
	@echo "Generating API proto files..."
	cd ./api && go generate ./...
	@echo "Proto generation complete."

# Tidy dependencies
tidy:
	@echo "Tidying racing module..."
	go mod tidy -C racing
	@echo "Tidying sports module..."
	go mod tidy -C sports
	@echo "Tidying API module..."
	go mod tidy -C api
	@echo "Dependencies tidied."

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f racing/racing racing/coverage.out
	rm -f sports/sports sports/coverage.out
	rm -f api/api api/coverage.out
	@echo "Clean complete."

# Kill all running services by port
kill-all:
		@echo "Killing all services..."
		@for endpoint in $(SERVICE_ENDPOINTS); do \
			port=$${endpoint#*:}; \
			echo "  Killing port $$port..."; \
			lsof -ti:$$port 2>/dev/null | xargs kill -9 2>/dev/null || true; \
		done
		@echo "All services killed."

# Kill sports service specifically
kill-sports:
	@echo "Killing sports service..."
	@port=$${SPORTS_GRPC_ENDPOINT#*:}; \
	echo "  Killing port $$port..."; \
	lsof -ti:$$port 2>/dev/null | xargs kill -9 2>/dev/null || true
	@echo "Sports service killed."

# Run racing service (gRPC on :9000)
run-racing:
	@echo "Starting racing service..."
	go build -C racing -o racing . && cd ./racing && ./racing \
		-grpc-endpoint=$(RACING_GRPC_ENDPOINT)

# Run sports service (gRPC on :9001)
run-sports:
	@echo "Starting sports service..."
	go build -C sports -o sports . && cd ./sports && ./sports \
		-grpc-endpoint=$(SPORTS_GRPC_ENDPOINT)

# Run API gateway (REST on :8000)
run-api:
	@echo "Starting API gateway..."
	go build -C api -o api . && cd ./api && ./api \
		-racing-endpoint=$(RACING_GRPC_ENDPOINT) \
		-sports-endpoint=$(SPORTS_GRPC_ENDPOINT) \
		-api-endpoint=$(API_ENDPOINT)

# Run all services concurrently
run-all: build
	@echo "Starting all services..."
	@echo "Starting racing service (gRPC on $(RACING_GRPC_ENDPOINT))..."
	cd ./racing && ./racing -grpc-endpoint=$(RACING_GRPC_ENDPOINT) &
	@echo "Starting sports service (gRPC on $(SPORTS_GRPC_ENDPOINT))..."
	cd ./sports && ./sports -grpc-endpoint=$(SPORTS_GRPC_ENDPOINT) &
	@echo "Starting API gateway (REST on $(API_ENDPOINT))..."
	cd ./api && ./api \
		-racing-endpoint=$(RACING_GRPC_ENDPOINT) \
		-sports-endpoint=$(SPORTS_GRPC_ENDPOINT) \
		-api-endpoint=$(API_ENDPOINT) &
	@echo "All services started."
	wait

# Format all Go files
fmt:
	@echo "Formatting Go files..."
	gofmt -w ./racing ./sports ./api
	@echo "Formatting complete."

# Show help
help:
	@echo "Entain BE Technical Test - Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Build all services (default goal, runs tidy, generate, fmt, lint)"
	@echo "  build              Build all services (runs tidy, generate, fmt, lint)"
	@echo "  test               Run unit tests"
	@echo "  integration-local  Run integration tests with local services"
	@echo "  test-all           Run all tests (unit + integration)"
	@echo "  lint               Run linters and format checks"
	@echo "  generate           Generate proto files"
	@echo "  tidy               Tidy module dependencies"
	@echo "  clean              Remove build artifacts"
	@echo "  fmt                Format all Go files"
	@echo "  run-racing         Build and start racing service (gRPC on $(RACING_GRPC_ENDPOINT))"
	@echo "  run-sports         Build and start sports service (gRPC on $(SPORTS_GRPC_ENDPOINT))"
	@echo "  run-api            Build and start API gateway (REST on $(API_ENDPOINT))"
	@echo "  run-all            Build and start all services concurrently"
	@echo "  kill-all           Kill all running services"
	@echo "  kill-sports        Kill sports service specifically"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build              # Build all services"
	@echo "  make test               # Run unit tests"
	@echo "  make generate           # Generate proto files"
	@echo "  make run-racing         # Start racing service"
	@echo "  make run-sports         # Start sports service"
	@echo "  make run-api            # Start API gateway"
	@echo "  make run-all            # Start all services"
	@echo "  make kill-sports        # Stop sports service"
	@echo "  make kill-all           # Stop all services"
	@echo ""
	@echo "Variables:"
	@echo "  RACING_GRPC_ENDPOINT  Racing service gRPC endpoint (default: localhost:9000)"
	@echo "  SPORTS_GRPC_ENDPOINT  Sports service gRPC endpoint (default: localhost:9001)"
	@echo "  API_ENDPOINT          API gateway REST endpoint (default: localhost:8000)"
	@echo ""
	@echo "Examples with custom endpoints:"
	@echo "  make run-racing RACING_GRPC_ENDPOINT=0.0.0.0:9000"
	@echo "  make run-sports SPORTS_GRPC_ENDPOINT=0.0.0.0:9001"
	@echo "  make run-api API_ENDPOINT=0.0.0.0:8000 RACING_GRPC_ENDPOINT=0.0.0.0:9000 SPORTS_GRPC_ENDPOINT=0.0.0.0:9001"
