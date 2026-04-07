# Task 2 Implementation Plan: Order Races by `advertised_start_time`

---

## Overview

This task implements ordering for the `ListRaces` endpoint, with races ordered by `advertised_start_time` by default. It also allows consumers to specify which field to sort by and the sort direction (ascending/descending).

### What This Accomplishes
- **Primary requirement**: Races are returned ordered by `advertised_start_time` by default
- **Bonus**: Consumers can specify a field to sort via `sort_by`
- **Bonus**: Consumers can specify sort direction via `descending` boolean
- **Backward compatible**: Existing clients get ordered results without changes

### How It Fits Into the System
- **API Layer**: HTTP clients can specify `sort_by` and `descending` in filter requests
- **Service Layer**: Passes filter parameters to the repository unchanged
- **Repository Layer**: Constructs ORDER BY clause with whitelist validation for security
- **Database**: SQLite executes the ordered query

---

## Changes Required

### 1. Racing Service Proto - `racing/proto/racing/racing.proto`

**File Path**: `entain-master/racing/proto/racing/racing.proto`

**Change Type**: Modify

**Current Code**:
```entain-master/racing/proto/racing/racing.proto#L25-30
// Filter for listing races.
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
}
```

**New Code**:
```entain-master/racing/proto/racing/racing.proto#L25-40
// Filter for listing races.
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
  // Optional field to specify which field to sort by.
  // Supported values: "advertised_start_time", "name", "id", "meeting_id", "number"
  // If not set, defaults to "advertised_start_time".
  optional string sort_by = 3;
  // Optional field to specify sort direction.
  // If true, sorts in descending order (latest first).
  // If false or not set, sorts in ascending order (earliest first).
  optional bool descending = 4;
}
```

**Purpose**: 
- Adds `sort_by` to allow consumers to specify sort field (bonus requirement)
- Adds `descending` boolean for clean API (clearer than enum for binary choice)
- Both fields are optional for backward compatibility

---

### 2. API Gateway Proto - `api/proto/racing/racing.proto`

**File Path**: `entain-master/api/proto/racing/racing.proto`

**Change Type**: Modify

**Current Code**:
```entain-master/api/proto/racing/racing.proto#L28-34
// Filter for listing races.
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
}
```

**New Code**:
```entain-master/api/proto/racing/racing.proto#L28-44
// Filter for listing races.
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  // Optional filter for visible races. If not set, returns both visible and invisible races.
  optional bool visible = 2;
  // Optional field to specify which field to sort by.
  // Supported values: "advertised_start_time", "name", "id", "meeting_id", "number"
  // If not set, defaults to "advertised_start_time".
  optional string sort_by = 3;
  // Optional field to specify sort direction.
  // If true, sorts in descending order (latest first).
  // If false or not set, sorts in ascending order (earliest first).
  optional bool descending = 4;
}
```

**Purpose**: 
- Keeps API gateway proto synchronized with racing service proto
- HTTP annotations remain unchanged

---

### 3. Racing Repository - `racing/db/races.go`

**File Path**: `entain-master/racing/db/races.go`

**Change Type**: Modify

**Current `List` and `applyFilter` functions** (lines 46-78):
```entain-master/racing/db/races.go#L46-78
func (r *racesRepo) List(filter *racing.ListRacesRequestFilter) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getRaceQueries()[racesList]

	query, args = r.applyFilter(query, filter)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRaces(rows)
}

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

**New Code**:
```entain-master/racing/db/races.go#L46-120
func (r *racesRepo) List(filter *racing.ListRacesRequestFilter) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getRaceQueries()[racesList]

	// Step 1: Apply WHERE clause (filtering)
	query, args = r.applyFilter(query, filter)

	// Step 2: Apply ORDER BY (sorting)
	// Separate from filtering for single responsibility
	query = r.applySorting(query, filter)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRaces(rows)
}

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

// applySorting adds ORDER BY clause based on filter parameters.
// Defaults to ordering by advertised_start_time ASC.
func (r *racesRepo) applySorting(query string, filter *racing.ListRacesRequestFilter) string {
	// Determine sort field (default to advertised_start_time)
	sortBy := "advertised_start_time"
	if filter != nil && filter.SortBy != nil && *filter.SortBy != "" {
		sortBy = r.validateSortField(*filter.SortBy)
	}

	// Determine sort direction (default ASC)
	direction := "ASC"
	if filter != nil && filter.Descending != nil && *filter.Descending {
		direction = "DESC"
	}

	return query + fmt.Sprintf(" ORDER BY %s %s", sortBy, direction)
}

// validateSortField ensures the sort field is valid to prevent SQL injection.
// Returns the field name if valid, otherwise returns default "advertised_start_time".
func (r *racesRepo) validateSortField(field string) string {
	// Whitelist of allowed sort fields
	allowedFields := map[string]bool{
		"advertised_start_time": true,
		"name":                  true,
		"id":                    true,
		"meeting_id":            true,
		"number":                true,
	}

	if allowedFields[field] {
		return field
	}
	// Return safe default if invalid field provided
	return "advertised_start_time"
}
```

**Purpose**: 
- Refactors `List` to call `applyFilter` and `applySorting` separately (**single responsibility principle**)
- `applyFilter` now only handles WHERE clause construction
- `applySorting` handles ORDER BY clause construction independently
- Adds `validateSortField` for security (prevents SQL injection)
- Always applies sorting (defaults to advertised_start_time ASC)
- Supports configurable sort field and direction

---

## Implementation Steps

| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | Add `sort_by` and `descending` fields to racing proto | `racing/proto/racing/racing.proto` | None |
| 2 | Add same changes to API gateway proto | `api/proto/racing/racing.proto` | Step 1 |
| 3 | Regenerate Go code from protos | Generated `.pb.go` files | Steps 1-2 |
| 4 | Add `applySorting` and `validateSortField` methods in repository | `racing/db/races.go` | Step 3 |
| 5 | Add unit tests for sorting functionality | `racing/tests/races_test.go` | Step 4 |
| 6 | Update integration tests for API sorting | `api/tests/races_integration_test.go` | Step 5 |
| 7 | Run tests and verify implementation | All test files | Steps 1-6 |

---

## API/Interface Changes

### Protobuf Changes

**Updated Filter Message**:
```protobuf
message ListRacesRequestFilter {
  repeated int64 meeting_ids = 1;
  optional bool visible = 2;
  optional string sort_by = 3;     // "advertised_start_time", "name", "id", etc.
  optional bool descending = 4;    // true = DESC, false/null = ASC
}
```

### Example API Calls

**Default order (advertised_start_time ascending):**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H "Content-Type: application/json" \
  -d \'{"filter": {}}\'
```

**Sort by advertised_start_time descending (upcoming races first):**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H "Content-Type: application/json" \
  -d \'{"filter": {"sort_by": "advertised_start_time", "descending": true}}\'
```

**Sort by name A-Z:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H "Content-Type: application/json" \
  -d \'{"filter": {"sort_by": "name"}}\'
```

**Sort by name Z-A:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H "Content-Type: application/json" \
  -d \'{"filter": {"sort_by": "name", "descending": true}}\'
```

**Combined with existing filters:**
```bash
curl -X POST "http://localhost:8000/v1/list-races" \
  -H "Content-Type: application/json" \
  -d \'{"filter": {"meeting_ids": [1, 2], "visible": true, "sort_by": "advertised_start_time", "descending": false}}\'
```

### Expected Input/Output

**Input**:
- `sort_by`: String, optional. Field name to sort by. Supported: `advertised_start_time`, `name`, `id`, `meeting_id`, `number`. Defaults to `advertised_start_time`.
- `descending`: Boolean, optional. If true, sorts in descending order. Defaults to false (ascending).

**Output**:
- Same `ListRacesResponse` structure
- Races ordered by specified field and direction
- Default order is `advertised_start_time ASC` when no sort parameters provided

---

## Testing Strategy

### Unit Tests Needed

**File**: `racing/tests/races_test.go`

**Test Cases**:
1. **Default sort ascending**: Verify races are ordered by advertised_start_time ASC when no sort params provided
2. **Default sort with filter**: Verify default sorting works when combined with other filters
3. **Explicit sort ascending**: Verify sort_by + descending=false orders correctly
4. **Explicit sort descending**: Verify sort_by + descending=true orders correctly
5. **Sort by name**: Verify alphabetical ordering works
6. **Invalid sort_by**: Verify invalid sort field falls back to safe default
7. **Sort with equal times**: Verify stable ordering for races with identical start times
8. **Combined filters and sort**: Verify sorting works with meeting_ids and visible filters

---

## Regeneration Steps

### Prerequisites
Ensure protoc plugins are installed:
```bash
cd entain-master/racing
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
  google.golang.org/genproto/googleapis/api \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc \
  google.golang.org/protobuf/cmd/protoc-gen-go
```

### Generate Racing Service Protos
```bash
cd entain-master/racing
go generate ./...
```

### Generate API Gateway Protos
```bash
cd entain-master/api
go generate ./...
```

---

## Potential Risks/Considerations

### Backward Compatibility
- All new fields are `optional`, existing clients continue to work
- Default behavior (sorted by advertised_start_time ASC) improves UX
- No changes to existing filter behavior

### Database Migration
- None required: Uses existing columns
- May want to add index on commonly sorted fields (advertised_start_time, name) for performance

### Performance Implications
- ORDER BY adds overhead to query execution
- For large datasets, sorting performance depends on indexes
- Sorting is required by spec, not optional

### Security Considerations
- `validateSortField` whitelist prevents SQL injection
- Only pre-approved fields can be used in ORDER BY clause
- Adding new sortable fields requires updating the whitelist

### Future Extensibility (Task 5)
- Sports service will follow same pattern with `sort_by` and `descending` fields
- Additional sort fields are easy to add to whitelist

---

## Summary

This implementation adds flexible sorting capability with `advertised_start_time` as the default sort field. The boolean `descending` field provides a clean API for consumers while maintaining full backward compatibility. The whitelist validation ensures security while allowing flexibility in sort fields.

**Estimated effort**: 2-3 hours including testing  
**Risk level**: Low (additive change with security validation)  
**Testing coverage**: Unit tests for repository layer, integration tests for API layer
