# Task 1 Implementation Plan: Add `visible` Filter to `ListRacesRequestFilter`

## Overview

This task adds a `visible` filter field to the `ListRacesRequestFilter` message, allowing API consumers to filter races by their visibility status. This is the first pending task from the technical test requirements.

**Key Changes:**
- Add `visible` field (as an optional bool) to the proto definition
- Update the repository layer to apply the visibility filter in SQL queries
- Update tests to cover the new filter functionality

**Impact:** This change affects both the `racing` gRPC service and the `api` REST gateway, as both share the proto definitions.

---

## Changes Required

### 1. Proto Definitions (Both Services)

#### File: `racing/proto/racing/racing.proto`
**Change Type:** Modify

```protobuf
# Before (line 28-30)
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
}
```

```protobuf
# After
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
}
```

#### File: `api/proto/racing/racing.proto`
**Change Type:** Modify (same change as above)

```protobuf
# Before (line 31-33)
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
}
```

```protobuf
# After
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
}
```

**Purpose:** Using `optional bool` allows us to distinguish between:
- `visible=true` (filter for visible races only)
- `visible=false` (filter for invisible races only)
- `visible=null/omitted` (no filtering by visibility)

**Note:** The project already has `--experimental_allow_proto3_optional` enabled in the `go:generate` directives, so `optional bool` is fully supported.

---

### 2. Repository Layer

#### File: `racing/db/races.go`
**Change Type:** Modify `applyFilter` method

```go
# Before (line 67-82)
func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
		}
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}
```

```go
# After
func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
		}
	}

	// Add visible filter if specified
	if filter.Visible != nil {
		clauses = append(clauses, "visible = ?")
		args = append(args, *filter.Visible)
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}
```

**Purpose:** Extends the filter logic to include visibility filtering when the `visible` field is set. Note that `optional bool` generates as `*bool` in Go, so we dereference with `*filter.Visible`.

---

### 3. Tests

#### File: `racing/tests/races_test.go`
**Change Type:** Add new test cases

Add import at top of file (if not already present):
```go
"google.golang.org/protobuf/proto"
```

Add the following test cases to the `TestListRaces_Service` test (within the `testCases` array):

```go
{
	name: "visible filter true",
	filter: &racingpb.ListRacesRequestFilter{
		Visible: proto.Bool(true),
	},
	expectedCount: 4, // Races 1, 3, 4, 6 are visible
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		for _, race := range races {
			if !race.Visible {
				t.Errorf("expected visible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
	seedData: true,
},
{
	name: "visible filter false",
	filter: &racingpb.ListRacesRequestFilter{
		Visible: proto.Bool(false),
	},
	expectedCount: 2, // Races 2, 5 are not visible
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		for _, race := range races {
			if race.Visible {
				t.Errorf("expected invisible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
	seedData: true,
},
{
	name: "visible filter with meeting ids",
	filter: &racingpb.ListRacesRequestFilter{
		MeetingIds: []int64{1},
		Visible:    proto.Bool(true),
	},
	expectedCount: 2, // Races 1, 3 from meeting 1 are visible
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		for _, race := range races {
			if race.MeetingId != 1 {
				t.Errorf("expected meeting_id 1, got %d", race.MeetingId)
			}
			if !race.Visible {
				t.Errorf("expected visible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
	seedData: true,
},
```

**Purpose:** Comprehensive test coverage for the new visibility filter including:
- Filtering for visible races only
- Filtering for invisible races only
- Combining visibility filter with existing meeting_ids filter

---

### 4. API Integration Tests

#### File: `api/tests/races_integration_test.go`
**Change Type:** Add new test cases

Add import at top of file (if not already present):
```go
"google.golang.org/protobuf/proto"
```

Add to the `TestAPI_ListRaces_EdgeCases` test (within the `testCases` array):

```go
{
	name: "visible filter true",
	filter: &racingpb.ListRacesRequestFilter{
		Visible: proto.Bool(true),
	},
	expectedCount: -1,
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		for _, race := range races {
			if !race.Visible {
				t.Errorf("expected visible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
},
{
	name: "visible filter false",
	filter: &racingpb.ListRacesRequestFilter{
		Visible: proto.Bool(false),
	},
	expectedCount: -1,
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		for _, race := range races {
			if race.Visible {
				t.Errorf("expected invisible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
},
{
	name: "visible filter with meeting ids",
	filter: &racingpb.ListRacesRequestFilter{
		MeetingIds: []int64{1},
		Visible:    proto.Bool(true),
	},
	expectedCount: -1,
	validateRaces: func(t *testing.T, races []*racingpb.Race) {
		assertRacesFromMeetings(t, races, []int64{1})
		for _, race := range races {
			if !race.Visible {
				t.Errorf("expected visible race, got race %d with visible=%v", race.Id, race.Visible)
			}
		}
	},
},
```

**Purpose:** End-to-end testing of the visibility filter through the REST API gateway.

---

## Implementation Steps

| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | ✅ Add `visible` field to proto in racing service | `racing/proto/racing/racing.proto` | None |
| 2 | [ ] Add `visible` field to proto in API gateway | `api/proto/racing/racing.proto` | None |
| 3 | [ ] Update repository filter logic | `racing/db/races.go` | Steps 1 & 2 |
| 4 | [ ] Regenerate protobuf code | Both services | Steps 1 & 2 |
| 5 | [ ] Add unit tests for visible filter | `racing/tests/races_test.go` | Steps 3 & 4 |
| 6 | [ ] Add integration tests | `api/tests/races_integration_test.go` | Steps 3 & 4 |
| 7 | [ ] Run tests and verify | Both services | Steps 5 & 6 |

---

## API/Interface Changes

### Proto Message Changes

**New Field:**
```protobuf
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  optional bool visible = 2;  // NEW
}
```

### Example API Calls

**Get only visible races:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {"visible": true}}'
```

**Get only invisible races:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {"visible": false}}'
```

**Combine with meeting_ids filter:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {"meeting_ids": [1, 2], "visible": true}}'
```

**No visibility filter (returns all):**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H 'Content-Type: application/json' \
  -d '{"filter": {"meeting_ids": [1]}}'
```

---

## Generated Code Comparison

**With `optional bool`:**
```go
// Generated Go code
type ListRacesRequestFilter struct {
	MeetingIds []int64
	Visible    *bool  // Pointer to bool for optional field
}

// Usage in filter logic:
if filter.Visible != nil {
    // Field was explicitly set
    if *filter.Visible {
        // filter for visible races
    }
}
```

**With `google.protobuf.BoolValue`:**
```go
// Generated Go code
type ListRacesRequestFilter struct {
	MeetingIds []int64
	Visible    *wrapperspb.BoolValue  // Pointer to wrapper type
}

// Usage in filter logic:
if filter.Visible != nil {
    // Field was explicitly set
    if filter.Visible.Value {
        // filter for visible races
    }
}
```

**Benefits of `optional bool`:**
- ✅ Cleaner generated code (no `.Value` accessor needed)
- ✅ No wrapper type import required
- ✅ Uses `proto.Bool()` helper for construction
- ✅ More idiomatic Go code

---

## Testing Strategy

### Unit Tests (`racing/tests/races_test.go`)

**Test Cases to Cover:**
1. `visible filter true` - Returns only visible races
2. `visible filter false` - Returns only invisible races
3. `visible filter with meeting ids` - Combined filtering
4. Existing tests should still pass (nil/empty filter)

**Follow Testing Guidelines from `CLAUDE.md`:**
- Use in-memory database (`:memory:`)
- Explicit data seeding via `seedData` field
- Table-driven tests with clear pass/fail per case
- Use `t.Helper()` in helper functions
- Use `t.Cleanup()` for teardown

### Integration Tests (`api/tests/races_integration_test.go`)

**Test Cases to Cover:**
1. HTTP endpoint accepts `visible` filter
2. Response validation for filtered results
3. Combined filters work correctly

---

## Regeneration Steps

After modifying proto files, regenerate the Go code:

```bash
# Regenerate racing service protos
cd racing
go generate ./...

# Regenerate API gateway protos
cd api
go generate ./...
```

This will:
- Generate updated `*.pb.go` files with the new `Visible` field (as `*bool`)
- Update gRPC service stubs
- Update grpc-gateway mappings for the API service

---

## Potential Risks/Considerations

### Backward Compatibility
✅ **Good:** Using `optional bool` means:
- Existing clients not sending the field will continue to work (returns all races)
- Field is optional and backward compatible
- No breaking changes to existing API consumers

### Database Migration
✅ **None required:** The `visible` column already exists in the database schema

### Performance Implications
✅ **Minimal:** Adding a simple boolean filter to the WHERE clause has negligible performance impact

### Edge Cases to Handle
1. ✅ **Nil filter** - Already handled, returns all races
2. ✅ **Empty filter** - Already handled, returns all races
3. ✅ **Only visible field set** - Filter by visibility only
4. ✅ **Combined filters** - Both meeting_ids and visible filters work together
5. ✅ **Database with no races** - Returns empty list (already tested)

### Code Quality
- Follow [Google API Design](https://cloud.google.com/apis/design) for filter patterns
- Follow [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- All code properly commented
- Standard method naming maintained

---

## Status

- [x] Step 1: Proto definitions updated (both services) - using `optional bool`
- [ ] Step 2: Regenerate protobuf code
- [ ] Step 3: Update repository filter logic
- [ ] Step 4: Add unit tests
- [ ] Step 5: Add integration tests
- [ ] Step 6: Run all tests and verify

---

## Summary

This implementation plan adds the `visible` filter to `ListRacesRequestFilter` using the modern `optional bool` approach:

1. **Proto-first approach** - Define the interface in protobuf with `optional` keyword
2. **Cleaner generated code** - `*bool` instead of `*wrapperspb.BoolValue`
3. **Leverages existing support** - Project already has `--experimental_allow_proto3_optional` enabled
4. **Comprehensive testing** - Unit and integration tests covering all scenarios
5. **Backward compatible** - No breaking changes for existing clients
6. **Follows project guidelines** - Adheres to all standards in `CLAUDE.md`

The change is straightforward and low-risk, making it an ideal first task for the technical test.