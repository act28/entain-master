# Task 3 Implementation Plan: Add Computed Status Field

## Overview

This task adds a computed `status` field to the `Race` resource that indicates whether a race is **OPEN** or **CLOSED** based on the current time relative to the race's `advertised_start_time`. The status is:
- **OPEN**: When the current time is before the `advertised_start_time`
- **CLOSED**: When the current time is at or after the `advertised_start_time`

This provides clients with a clear indication of whether a race is still available for participation.

---

## Changes Required

### 1. Racing Proto - Add Status Enum and Field

**File**: `racing/proto/racing/racing.proto`
**Change Type**: Modify

Add a `Status` enum and add the `status` field to the `Race` message:

```protobuf
/* Resources */

// A race resource.
message Race {
  // Status represents the current state of a race.
  enum Status {
    // UNSPECIFIED indicates an unknown status.
    UNSPECIFIED = 0;
    // OPEN indicates the race has not yet started.
    OPEN = 1;
    // CLOSED indicates the race has started or finished.
    CLOSED = 2;
  }
  // ID represents a unique identifier for the race.
  int64 id = 1;
  // MeetingID represents a unique identifier for the races meeting.
  int64 meeting_id = 2;
  // Name is the official name given to the race.
  string name = 3;
  // Number represents the number of the race.
  int64 number = 4;
  // Visible represents whether or not the race is visible.
  bool visible = 5;
  // AdvertisedStartTime is the time the race is advertised to run.
  google.protobuf.Timestamp advertised_start_time = 6;
  // Status represents whether the race is open or closed for betting.
  // Computed based on advertised_start_time vs current time.
  Status status = 7;
}
```

**Purpose**: Define the Status enum and add it to the Race message so it's included in the API response.

---

### 2. API Proto - Add Status Enum and Field

**File**: `api/proto/racing/racing.proto`
**Change Type**: Modify

Add the same Status enum and field to the API proto (for grpc-gateway):

```protobuf
/* Resources */

// A race resource.
message Race {
  // Status represents the current state of a race.
  enum Status {
    // UNSPECIFIED indicates an unknown status.
    UNSPECIFIED = 0;
    // OPEN indicates the race has not yet started.
    OPEN = 1;
    // CLOSED indicates the race has started or finished.
    CLOSED = 2;
  }
  // ID represents a unique identifier for the race.
  int64 id = 1;
  // MeetingID represents a unique identifier for the races meeting.
  int64 meeting_id = 2;
  // Name is the official name given to the race.
  string name = 3;
  // Number represents the number of the race.
  int64 number = 4;
  // Visible represents whether or not the race is visible.
  bool visible = 5;
  // AdvertisedStartTime is the time the race is advertised to run.
  google.protobuf.Timestamp advertised_start_time = 6;
  // Status represents whether the race is open or closed for betting.
  // Computed based on advertised_start_time vs current time.
  Status status = 7;
}
```

**Purpose**: Keep the API proto in sync with the racing service proto for proper REST API responses.

---

### 3. Repository - Compute Status in scanRaces

**File**: `racing/db/races.go`
**Change Type**: Modify

Update the `scanRaces` method to compute and set the status based on current time:

```go
func (m *racesRepo) scanRaces(
	rows *sql.Rows,
) ([]*racing.Race, error) {
	var races []*racing.Race

	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
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

		races = append(races, &race)
	}

	return races, nil
}
```

**Purpose**: Add status computation during race scanning from database results.

---

### 4. Repository - Add Status Computation Helper

**File**: `racing/db/races.go`
**Change Type**: Add

Add a helper function to compute the status:

```go
// computeRaceStatus determines if a race is OPEN or CLOSED based on the 
// advertised start time compared to the current time.
// 
// A race is:
// - OPEN if the current time is before the advertised start time
// - CLOSED if the current time is at or after the advertised start time
func computeRaceStatus(advertisedStart time.Time) racing.Race_Status {
	// Ensure consistent timezone handling by using UTC for comparison
	if time.Now().UTC().Before(advertisedStart.UTC()) {
		return racing.Race_Status_OPEN
	}
	return racing.Race_Status_CLOSED
}
```

**Purpose**: Centralize status computation logic with clear documentation.

---

### 5. Tests - Add Status Test Cases

**File**: `racing/tests/races_test.go`
**Change Type**: Modify

Add new test cases to `TestListRaces_Service` to validate status computation if necessary. The test data should contain races with both negative (past) and positive (future) time offsets:

```go
		{
			name: "validate open race status",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{2}, // Meeting 2 has only future races
			},
			expectedCount: 2,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.Status != racingpb.Race_OPEN {
						t.Errorf("expected status OPEN for future race %d, got %v", race.Id, race.Status)
					}
				}
			},
			seedData: true,
		},
		{
			name: "validate closed race status",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1}, // Meeting 1 has past and future races
			},
			expectedCount: 3,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					switch race.Id {
					case 1, 2: // Past races should be CLOSED
						if race.Status != racingpb.Race_CLOSED {
							t.Errorf("expected status CLOSED for past race %d, got %v", race.Id, race.Status)
						}
					case 3: // Future race should be OPEN
						if race.Status != racingpb.Race_OPEN {
							t.Errorf("expected status OPEN for future race %d, got %v", race.Id, race.Status)
						}
					}
				}
			},
			seedData: true,
		},
```

**Purpose**: Validate that races with future times show OPEN status and past times show CLOSED status.

---

### 6. Tests - Ensure Test Data Has Mixed Time Offsets

**File**: `racing/tests/races_test.go`
**Change Type**: Verify/Modify

Ensure the `defaultTestRaces` slice in `TestListRaces_Service` includes a **mix of negative (past) and positive (future) time offsets**. This allows tests to verify both OPEN and CLOSED status values:

```go
defaultTestRaces := []testRace{
	{ID: 1, MeetingID: 1, Name: "Race 1", Number: 1, Visible: true, AdvertisedStartTime: now.Add(-5 * time.Hour)},   // Past - CLOSED
	{ID: 2, MeetingID: 1, Name: "Race 2", Number: 2, Visible: false, AdvertisedStartTime: now.Add(-2 * time.Hour)}, // Past - CLOSED
	{ID: 3, MeetingID: 1, Name: "Race 3", Number: 3, Visible: true, AdvertisedStartTime: now.Add(1 * time.Hour)},     // Future - OPEN
	{ID: 4, MeetingID: 2, Name: "Race 4", Number: 1, Visible: true, AdvertisedStartTime: now.Add(3 * time.Hour)},  // Future - OPEN
	{ID: 5, MeetingID: 2, Name: "Race 5", Number: 2, Visible: false, AdvertisedStartTime: now.Add(6 * time.Hour)}, // Future - OPEN
	{ID: 6, MeetingID: 3, Name: "Race 6", Number: 1, Visible: true, AdvertisedStartTime: now.Add(9 * time.Hour)},  // Future - OPEN
}
```

**Purpose**: Provide test data with mixed times to verify OPEN/CLOSED status computation. The data is seeded via `seedTestData(t, db, defaultTestRaces)` inside each test case where `seedData: true`.

---

## Implementation Steps

| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | Add Status enum and field to racing proto | `racing/proto/racing/racing.proto` | None |
| 2 | Add Status enum and field to API proto | `api/proto/racing/racing.proto` | Step 1 |
| 3 | Regenerate protobuf files | All generated `*.pb.go` files | Steps 1-2 |
| 4 | Add status computation helper function | `racing/db/races.go` | Step 3 |
| 5 | Update scanRaces to compute and set status | `racing/db/races.go` | Step 4 |
| 6 | Verify test data has mixed time offsets (past/future) | `racing/tests/races_test.go` | None |
| 7 | Add status validation test cases | `racing/tests/races_test.go` | Step 6 |
| 8 | Run tests to verify implementation | Test output | Steps 1-7 |

---

## API/Interface Changes

### New Protobuf Enum

```protobuf
// Status represents the current state of a race.
enum Status {
  // UNSPECIFIED indicates an unknown status.
  UNSPECIFIED = 0;
  // OPEN indicates the race has not yet started.
  OPEN = 1;
  // CLOSED indicates the race has started or finished.
  CLOSED = 2;
}
```

**Note**: The Status enum is nested inside the Race message to follow Google Protocol Buffer style guidelines. Nesting provides scoping that allows clean enum value names without the `STATUS_` prefix, while avoiding global namespace collisions.

### Updated Race Message

The `Race` message now includes `status` field (index 7) using the nested Status enum type.

### Example API Response

```json
{
  "races": [
    {
      "id": "1",
      "meeting_id": "1",
      "name": "Race 1",
      "number": 1,
      "visible": true,
      "advertised_start_time": "2024-01-15T10:00:00Z",
      "status": "OPEN"
    },
    {
      "id": "2",
      "meeting_id": "1",
      "name": "Race 2",
      "number": 2,
      "visible": true,
      "advertised_start_time": "2024-01-10T10:00:00Z",
      "status": "CLOSED"
    }
  ]
}
```

### Example curl Command

```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {}}'
```

---

## Testing Strategy

### Unit Tests Needed

1. **Status computation logic** (`racing/db/races.go:computeRaceStatus`)
   - Future time returns `Race_Status_OPEN`
   - Past time returns `Race_Status_CLOSED`
   - Current exact time returns `Race_Status_CLOSED`

2. **Integration via service layer** (`racing/tests/races_test.go`)
   - Races with future `advertised_start_time` have OPEN status
   - Races with past `advertised_start_time` have CLOSED status

### Test Cases to Cover

| Scenario | Input | Expected Status |
|----------|-------|----------------|
| Race in the future | advertised_start_time = now + 1 hour | OPEN |
| Race in the past | advertised_start_time = now - 1 hour | CLOSED |
| Race at exact current time | advertised_start_time = now | CLOSED |

### Test Implementation

Following `CLAUDE.md` Testing Guidelines:
- Use in-memory SQLite database (`:memory:`)
- Explicit data seeding with controlled times
- Table-driven tests with clear pass/fail per case
- End-to-end flow through service layer

---

## Regeneration Steps

### 1. Racing Service Proto Generation

```bash
cd racing
go generate ./...
```

### 2. API Gateway Proto Generation

```bash
cd api
go generate ./...
```

---

## Potential Risks/Considerations

### Backward Compatibility
- **Risk**: Adding a new field is backward compatible for protobuf (clients can ignore unknown fields)
- **Mitigation**: New field has a default value of `UNSPECIFIED` (0) if not set

### Database Migration
- **Risk**: No database schema changes required - status is computed at read time
- **Benefit**: No migration needed, pure code change

### Performance Implications
- **Risk**: Status computation happens for every race on every query (calls `time.Now()`)
- **Mitigation**: Computation is trivial (single time comparison), minimal overhead
- **Alternative**: Could cache status if needed, but likely unnecessary

### Edge Cases
- **Exact time match**: When `advertised_start_time == now`, status is CLOSED (race has started)
- **Clock skew**: Status depends on server's system clock - ensure NTP sync in production
- **Timezone handling**: All times must be in consistent timezone (if code uses UTC explicitly, then database times should also be in UTC)

### Timezone Consistency (New)
- **Risk**: Status computation requires both current time and advertised_start_time to be in consistent timezone
- **Current fix**: Code explicitly converts both times to UTC before comparison
- **Database requirement**: Ensure advertised_start_time values in database are stored as UTC
- **Verification needed**: Confirm existing database data uses UTC, or migration may be required

### Testing Considerations
- Tests must use controlled times to avoid race conditions
- Use relative times (`now + duration`) rather than absolute timestamps
- Ensure test data includes both future and past races for comprehensive coverage

### Style Guide Considerations

**Enum Naming Convention Trade-offs**

The current implementation uses a nested enum with unprefixed value names:

```protobuf
message Race {
  enum Status {
    UNSPECIFIED = 0;
    OPEN = 1;
    CLOSED = 2;
  }
}
```

This approach deviates from the strict Google Protocol Buffer Style Guide which recommends prefixing all enum values with the enum name (e.g., `STATUS_OPEN`, `STATUS_CLOSED`). The nesting provides scoping that avoids global namespace collisions, which is one of the two recommended approaches in the style guide (prefixing all values OR nesting inside a message).

**Trade-offs:**
- **Current approach**: Clean JSON API response (`"status": "OPEN"`) but partial adherence to Google style
- **Strict Google standard**: Would use `STATUS_UNSPECIFIED`, `STATUS_OPEN`, `STATUS_CLOSED` resulting in JSON response `"status": "STATUS_OPEN"` (verbose but fully compliant)

**Rationale**: The task explicitly requires `OPEN`/`CLOSED` in the API response. Nesting the enum achieves this while still following the structural guidance of the style guide (using message scoping to avoid collisions). This is a pragmatic compromise between style compliance and API usability.

**Alternative approaches** (if strict compliance is required later):
1. Use top-level enum with prefixed values (`STATUS_OPEN`, `STATUS_CLOSED`) and accept verbose JSON
2. Use custom JSON marshaler to strip `STATUS_` prefix at the API layer while keeping prefixed values in proto (complexity and maintenance trade-offs)
3. Define the API proto separately with a string field and map between internal enum and external string (also has complexity and maintenance trade-offs)
