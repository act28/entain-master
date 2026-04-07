# Task 4 Implementation Plan: Get Race RPC

## Overview

This task implements a `GetRace` RPC method to retrieve a single race by its unique ID. This complements the existing `ListRaces` functionality by providing direct access to specific race resources, following the standard Google API design pattern for resource retrieval (Get method).

**Key Objectives:**
- Add `GetRace` RPC to the Racing service
- Implement repository method to fetch a single race by ID
- Add HTTP endpoint for REST API access
- Compute `status` field dynamically based on advertised_start_time
- Follow existing code patterns and testing guidelines

---

## Changes Required

### 1. Proto Definition - Racing Service

**File Path:** `racing/proto/racing/racing.proto`

**Change Type:** Add

**Code Snippet:**
```racing/proto/racing/racing.proto#L12-16
service Racing {
  // ListRaces will return a collection of all races.
  rpc ListRaces(ListRacesRequest) returns (ListRacesResponse) {}

  // GetRace returns a single race by its ID.
  rpc GetRace(GetRaceRequest) returns (GetRaceResponse) {}
}
```

**Purpose:** Define the new RPC method in the core gRPC service.

---

**File Path:** `racing/proto/racing/racing.proto`

**Change Type:** Add

**Code Snippet:**
```racing/proto/racing/racing.proto#L32-42
/* GetRace Request/Response */

// Request for GetRace call.
message GetRaceRequest {
  // The unique ID of the race to retrieve.
  int64 id = 1;
}

// Response to GetRace call.
message GetRaceResponse {
  // The requested race resource.
  Race race = 1;
}
```

**Purpose:** Define request and response messages for GetRace RPC.

---

### 2. Proto Definition - API Gateway

**File Path:** `api/proto/racing/racing.proto`

**Change Type:** Add

**Code Snippet:**
```api/proto/racing/racing.proto#L12-20
service Racing {
  // ListRaces returns a list of all races.
  rpc ListRaces(ListRacesRequest) returns (ListRacesResponse) {
    option (google.api.http) = {
      post: "/v1/list-races"
      body: "*"
    };
  }

  // GetRace returns a single race by its ID.
  rpc GetRace(GetRaceRequest) returns (GetRaceResponse) {
    option (google.api.http) = {
      get: "/v1/races/{id}"
    };
  }
}
```

**Purpose:** Add GetRace RPC to gateway proto with HTTP GET annotation following REST conventions (GET /v1/races/{id}).

---

**File Path:** `api/proto/racing/racing.proto`

**Change Type:** Add

**Code Snippet:**
```api/proto/racing/racing.proto#L40-50
/* GetRace Request/Response */

// Request for GetRace call.
message GetRaceRequest {
  // The unique ID of the race to retrieve.
  int64 id = 1;
}

// Response to GetRace call.
message GetRaceResponse {
  // The requested race resource.
  Race race = 1;
}
```

**Purpose:** Define request and response messages (must mirror racing proto).

---

### 3. Repository Layer

**File Path:** `racing/db/queries.go`

**Change Type:** Add

**Code Snippet:**
```racing/db/queries.go#L4-5
const (
	racesList = "list"
	racesGet  = "get"  // Add this constant
)
```

**Purpose:** Add constant for GetRace query identifier.

---

**File Path:** `racing/db/queries.go`

**Change Type:** Add

**Code Snippet:**
```racing/db/queries.go#L16-25
func getRaceQueries() map[string]string {
	return map[string]string{
		racesList: `
			SELECT
				id,
				meeting_id,
				name,
				number,
				visible,
				advertised_start_time
			FROM races
		`,
		racesGet: `
			SELECT
				id,
				meeting_id,
				name,
				number,
				visible,
				advertised_start_time
			FROM races
			WHERE id = ?
		`,
	}
}
```

**Purpose:** Add SQL query for fetching a single race by ID.

---

**File Path:** `racing/db/races.go`

**Change Type:** Add

**Code Snippet:**
```racing/db/races.go#L20-23
// RacesRepo provides repository access to races.
type RacesRepo interface {
	// Init will initialise our races repository.
	Init() error

	// List will return a list of races.
	List(filter *racing.ListRacesRequestFilter) ([]*racing.Race, error)

	// Get returns a single race by its ID.
	// Returns nil, nil if the race is not found.
	Get(id int64) (*racing.Race, error)
}
```

**Purpose:** Add Get method signature to the RacesRepo interface.

---

**File Path:** `racing/db/races.go`

**Change Type:** Add

**Code Snippet:**
```racing/db/races.go#L130-175
// Get returns a single race by its ID.
// Returns nil, nil if no race is found with the given ID.
func (r *racesRepo) Get(id int64) (*racing.Race, error) {
	query := getRaceQueries()[racesGet]

	row := r.db.QueryRow(query, id)

	var race racing.Race
	var advertisedStart time.Time

	err := row.Scan(
		&race.Id,
		&race.MeetingId,
		&race.Name,
		&race.Number,
		&race.Visible,
		&advertisedStart,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	ts, err := ptypes.TimestampProto(advertisedStart)
	if err != nil {
		return nil, err
	}

	race.AdvertisedStartTime = ts

	// Compute status based on advertised_start_time vs current time
	race.Status = computeRaceStatus(advertisedStart)

	return &race, nil
}
```

**Purpose:** Implement Get method to fetch a single race by ID, reusing existing computeRaceStatus helper.

---

### 4. Service Layer

**File Path:** `racing/service/racing.go`

**Change Type:** Modify

**Code Snippet:**
```racing/service/racing.go#L10-15
type Racing interface {
	// ListRaces will return a collection of races.
	ListRaces(ctx context.Context, in *racing.ListRacesRequest) (*racing.ListRacesResponse, error)

	// GetRace returns a single race by its ID.
	GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.GetRaceResponse, error)
}
```

**Purpose:** Add GetRace method to Racing interface.

---

**File Path:** `racing/service/racing.go`

**Change Type:** Add

**Code Snippet:**
```racing/service/racing.go#L35-55
func (s *racingService) GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.GetRaceResponse, error) {
	race, err := s.racesRepo.Get(in.Id)
	if err != nil {
		return nil, err
	}

	if race == nil {
		// Return nil race when not found
		return &racing.GetRaceResponse{}, nil
	}

	return &racing.GetRaceResponse{Race: race}, nil
}
```

**Purpose:** Implement GetRace service method that delegates to repository.

---

### 5. Main (Optional - if racing service needs updating)

**File Path:** `racing/main.go`

**Change Type:** No changes required (existing registration covers new RPC automatically)

**Purpose:** The gRPC server automatically serves all RPCs defined in the Racing service; no changes needed.

---

## Implementation Steps

| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | Add GetRace RPC and messages to racing proto | `racing/proto/racing/racing.proto` | None |
| 2 | Add GetRace SQL query constant | `racing/db/queries.go` | None |
| 3 | Add Get method to RacesRepo interface and implementation | `racing/db/races.go` | Step 2 |
| 4 | Add GetRace method to Racing interface and implementation | `racing/service/racing.go` | Step 3 |
| 5 | Regenerate Go code from racing proto | `racing/` directory | Step 1 |
| 6 | Add GetRace RPC and messages to API proto | `api/proto/racing/racing.proto` | Step 1 (same proto structure) |
| 7 | Regenerate Go code from API proto | `api/` directory | Step 6 |
| 8 | Add unit tests for Get repository method | `racing/db/races_test.go` (create if doesn't exist) or `racing/tests/races_test.go` | Steps 3, 4 |
| 9 | Add service layer tests for GetRace | `racing/tests/races_test.go` | Step 4 |
| 10 | Add integration tests for API endpoint | `api/tests/races_integration_test.go` | Step 7 |

---

## API/Interface Changes

### Protobuf Messages

**New Messages:**

```protobuf
// Request for GetRace call.
message GetRaceRequest {
  // The unique ID of the race to retrieve.
  int64 id = 1;
}

// Response to GetRace call.
message GetRaceResponse {
  // The requested race resource.
  Race race = 1;
}
```

### REST API Endpoint

**Endpoint:** `GET /v1/races/{id}`

**Example curl Commands:**

```bash
# Get race by ID (successful)
curl -X GET "http://localhost:8000/v1/races/1" \
  -H 'Content-Type: application/json'

# Expected response (200 OK):
{
  "race": {
    "id": 1,
    "meeting_id": 1,
    "name": "Race 1",
    "number": 1,
    "visible": true,
    "advertised_start_time": "2025-01-20T10:00:00Z",
    "status": "OPEN"
  }
}

# Get non-existent race
curl -X GET "http://localhost:8000/v1/races/9999" \
  -H 'Content-Type: application/json'

# Expected response (200 OK, but empty race):
{
  "race": null
}

# Get race with invalid ID (non-numeric)
curl -X GET "http://localhost:8000/v1/races/invalid" \
  -H 'Content-Type: application/json'

# Expected response (400 Bad Request)
```

### gRPC Service Method

```protobuf
rpc GetRace(GetRaceRequest) returns (GetRaceResponse)
```

---

## Testing Strategy

### Unit Tests - Repository Layer (`racing/db/races_test.go` or `racing/tests/races_test.go`)

**Test Cases:**

| Test Case | Description | Expected Result |
|-----------|-------------|-----------------|
| Get existing race | Call Get with valid race ID | Returns race with correct fields |
| Get non-existent race | Call Get with ID that doesn't exist | Returns nil, nil (no error) |
| Get race zero ID | Call Get with ID = 0 | Returns nil, nil |
| Get race negative ID | Call Get with negative ID | Returns nil, nil (or error) |
| Verify status field | Verify returned race has computed status | Status is OPEN or CLOSED based on time |
| Verify all fields | Verify all race fields are populated | All fields match database values |

**Example Test Structure:**
```racing/tests/races_test.go
func TestGetRace_Repo(t *testing.T) {
    testCases := []struct {
        name       string
        raceID     int64
        seedData   []testRace
        expectRace bool
        validate   func(t *testing.T, race *racingpb.Race)
    }{
        // Test cases...
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            db, cleanup := setupTestDB(t)
            t.Cleanup(cleanup)
            
            if len(tc.seedData) > 0 {
                seedTestData(t, db, tc.seedData)
            }
            
            repo := racingdb.NewRacesRepo(db)
            race, err := repo.Get(tc.raceID)
            
            if err != nil {
                t.Fatalf("Get failed: %v", err)
            }
            
            if tc.expectRace && race == nil {
                t.Fatal("expected race, got nil")
            }
            if !tc.expectRace && race != nil {
                t.Fatalf("expected nil, got race: %v", race)
            }
            
            if tc.validate != nil && race != nil {
                tc.validate(t, race)
            }
        })
    }
}
```

### Unit Tests - Service Layer (`racing/tests/races_test.go`)

**Test Cases:**

| Test Case | Description | Expected Result |
|-----------|-------------|-----------------|
| Get existing race | Call GetRace with valid ID | Returns GetRaceResponse with race |
| Get non-existent race | Call GetRace with invalid ID | Returns GetRaceResponse with nil race |
| Get race error | Repository returns error | Error propagated to caller |
| Context handling | Pass valid context | Context properly handled |

**Example Test Structure:**
```racing/tests/races_test.go
func TestGetRace_Service(t *testing.T) {
    testCases := []struct {
        name           string
        raceID         int64
        seedData       []testRace
        expectRace     bool
        expectError    bool
    }{
        // Test cases...
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            db, cleanup := setupTestDB(t)
            t.Cleanup(cleanup)
            
            if len(tc.seedData) > 0 {
                seedTestData(t, db, tc.seedData)
            }
            
            repo := racingdb.NewRacesRepo(db)
            racingService := service.NewRacingService(repo)
            
            request := &racingpb.GetRaceRequest{
                Id: tc.raceID,
            }
            
            response, err := racingService.GetRace(context.Background(), request)
            
            if tc.expectError {
                if err == nil {
                    t.Fatal("expected error, got nil")
                }
                return
            }
            
            if err != nil {
                t.Fatalf("GetRace failed: %v", err)
            }
            
            if tc.expectRace && response.Race == nil {
                t.Fatal("expected race in response, got nil")
            }
            if !tc.expectRace && response.Race != nil {
                t.Fatalf("expected nil race, got: %v", response.Race)
            }
        })
    }
}
```

### Integration Tests - API Layer (`api/tests/races_integration_test.go`)

**Test Cases:**

| Test Case | Description | Expected Result |
|-----------|-------------|-----------------|
| HTTP GET success | GET /v1/races/1 | 200 OK with race JSON |
| HTTP GET not found | GET /v1/races/9999 | 200 OK with null race |
| HTTP POST not allowed | POST /v1/races/1 | 405 Method Not Allowed |
| Invalid ID format | GET /v1/races/abc | 400 Bad Request |
| Response field validation | Verify all fields in response | All fields present and correct |
| Status field in response | Verify status is computed | Status is OPEN or CLOSED |

**Example Test Structure:**
```api/tests/races_integration_test.go
func TestAPI_GetRace_HTTP(t *testing.T) {
    testCases := []struct {
        name           string
        method         string
        raceID         string
        expectedStatus int
        expectRace     bool
    }{
        {
            name:           "GET existing race",
            method:         http.MethodGet,
            raceID:         "1",
            expectedStatus: http.StatusOK,
            expectRace:     true,
        },
        {
            name:           "GET non-existent race",
            method:         http.MethodGet,
            raceID:         "9999",
            expectedStatus: http.StatusOK,
            expectRace:     false,
        },
        {
            name:           "POST not allowed",
            method:         http.MethodPost,
            raceID:         "1",
            expectedStatus: http.StatusMethodNotAllowed,
            expectRace:     false,
        },
        // More test cases...
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            endpoint := fmt.Sprintf("%s/v1/races/%s", getAPIEndpoint(), tc.raceID)
            
            req, err := http.NewRequest(tc.method, endpoint, nil)
            if err != nil {
                t.Fatalf("failed to create request: %v", err)
            }
            
            client := &http.Client{Timeout: 10 * time.Second}
            resp, err := client.Do(req)
            if err != nil {
                t.Fatalf("failed to send request: %v", err)
            }
            defer resp.Body.Close()
            
            if resp.StatusCode != tc.expectedStatus {
                body, _ := io.ReadAll(resp.Body)
                t.Errorf("expected status %d, got %d. Body: %s", 
                    tc.expectedStatus, resp.StatusCode, string(body))
            }
        })
    }
}
```

---

## Regeneration Steps

### 1. Racing Service Proto Generation

**Command:**
```bash
cd racing
go generate ./...
```

**Expected Output:**
- `racing/racing.pb.go` - Updated with GetRace RPC and messages
- `racing/racing_grpc.pb.go` - Updated with GetRace client/server interfaces

### 2. API Gateway Proto Generation

**Command:**
```bash
cd api
go generate ./...
```

**Expected Output:**
- `racing/racing.pb.go` - Updated with GetRace RPC and messages
- `racing/racing_grpc.pb.go` - Updated with GetRace client interfaces
- `racing/racing.pb.gw.go` - HTTP gateway handler for GET /v1/races/{id}

### 3. Verify Generation

**Command:**
```bash
# Verify generated files exist and contain GetRace
grep -n "GetRace" racing/racing/*.go
grep -n "GetRace" api/racing/*.go
```

---

## Potential Risks/Considerations

### Backward Compatibility
- **Risk:** Low - This is a new endpoint, no existing functionality is modified
- **Mitigation:** Ensure existing `ListRaces` behavior remains unchanged

### Database Performance
- **Risk:** Low - Single-row lookup by primary key is indexed and fast
- **Mitigation:** No changes needed; query uses primary key index

### Error Handling
- **Risk:** Medium - Need consistent handling of not-found cases
- **Mitigation:** 
  - Return empty response (nil race) for not found rather than error
  - This matches common gRPC/REST patterns where 404 is optional for simplicity
  - Alternatively, could return NOT_FOUND status code (discuss with team)

### Timezone Handling
- **Risk:** Low - Status computation uses UTC time comparison
- **Mitigation:** Reuse existing `computeRaceStatus` function which already handles UTC

### Proto Consistency
- **Risk:** Medium - Changes must be kept in sync between racing/ and api/ protos
- **Mitigation:** Ensure both proto files have identical message definitions

### Testing Coverage
- **Risk:** Medium - Need comprehensive tests for edge cases
- **Mitigation:**
  - Test ID 0, negative IDs, very large IDs
  - Test with and without seed data
  - Test status computation with past/future times
  - Test HTTP integration with various path parameters

### API Design Consistency
- **Risk:** Low - Using GET /v1/races/{id} follows REST conventions
- **Mitigation:** Ensure documentation reflects this is a resource-oriented endpoint

---

## Documentation Requirements

1. **Update API documentation** to include GetRace endpoint
2. **Update proto comments** to document expected behavior
3. **Add curl examples** for common use cases
4. **Document error cases** and expected responses

---

## Post-Implementation Checklist

- [ ] Proto definitions updated in both racing/ and api/
- [ ] Go code regenerated in both services
- [ ] Repository layer Get method implemented and tested
- [ ] Service layer GetRace method implemented and tested
- [ ] Unit tests pass (`go test ./...` in racing/)
- [ ] Integration tests pass (`go test ./...` in api/)
- [ ] Manual curl test successful
- [ ] Code follows Uber Go Style Guide
- [ ] Documentation updated

---

## References

- [Google API Design Guide - Get method](https://cloud.google.com/apis/design/standard_methods#get)
- [CLAUDE.md - Project standards and testing guidelines](CLAUDE.md)
- [racing/db/races.go#L85-110](racing/db/races.go#L85-110) - computeRaceStatus implementation
- [racing/db/races.go#L45-60](racing/db/races.go#L45-60) - applyFilter pattern (reference for Get implementation)
