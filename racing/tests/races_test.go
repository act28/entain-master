package tests

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	racingdb "git.neds.sh/matty/entain/racing/db"
	racingpb "git.neds.sh/matty/entain/racing/proto/racing"
	"git.neds.sh/matty/entain/racing/service"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"
)

// TestListRaces_Service tests the ListRaces service method with various filters
func TestListRaces_Service(t *testing.T) {
	testCases := []struct {
		name          string
		filter        *racingpb.ListRacesRequestFilter
		expectedCount int
		validateRaces func(t *testing.T, races []*racingpb.Race)
		seedData      bool
	}{
		{
			name:          "nil filter",
			filter:        nil,
			expectedCount: 6,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
			seedData:      true,
		},
		{
			name:          "empty filter",
			filter:        &racingpb.ListRacesRequestFilter{},
			expectedCount: 6,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
			seedData:      true,
		},
		{
			name:          "empty database",
			filter:        &racingpb.ListRacesRequestFilter{},
			expectedCount: 0,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 0 {
					t.Fatalf("expected no races from empty database, got %d", len(races))
				}
			},
			seedData: false,
		},
		{
			name: "empty meeting ids",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{},
			},
			expectedCount: 6,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
			seedData:      true,
		},
		{
			name: "non-existent meeting ids",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{99999},
			},
			expectedCount: 0,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 0 {
					t.Errorf("expected no races for non-existent meeting, got %d", len(races))
				}
			},
			seedData: true,
		},
		{
			name: "single meeting id",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1},
			},
			expectedCount: 3, // Races 1, 2, 3
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.MeetingId != 1 {
						t.Errorf("expected meeting_id 1, got %d", race.MeetingId)
					}
				}
			},
			seedData: true,
		},
		{
			name: "multiple meeting ids",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 2},
			},
			expectedCount: 5, // Races 1-5
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.MeetingId != 1 && race.MeetingId != 2 {
						t.Errorf("expected meeting_id 1 or 2, got %d", race.MeetingId)
					}
				}
			},
			seedData: true,
		},
		{
			name: "mixed existing and non-existing meeting ids",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 99999},
			},
			expectedCount: 3, // Only races from meeting 1
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.MeetingId != 1 {
						t.Errorf("expected meeting_id 1, got %d", race.MeetingId)
					}
				}
			},
			seedData: true,
		},
		{
			name: "duplicate meeting ids in filter",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 1, 1},
			},
			expectedCount: 3, // Races 1, 2, 3 (duplicates should not affect results)
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.MeetingId != 1 {
						t.Errorf("expected meeting_id 1, got %d", race.MeetingId)
					}
				}
			},
			seedData: true,
		},
		{
			name:          "validate race fields",
			filter:        &racingpb.ListRacesRequestFilter{},
			expectedCount: 6,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				for _, race := range races {
					if race.Id == 0 {
						t.Error("expected non-zero race ID")
					}
					if race.MeetingId == 0 {
						t.Error("expected non-zero meeting_id")
					}
					if race.Name == "" {
						t.Error("expected non-empty race name")
					}
					if race.Number == 0 {
						t.Error("expected non-zero race number")
					}
					if race.AdvertisedStartTime == nil {
						t.Error("expected advertised_start_time to be set")
					}
				}
			},
			seedData: true,
		},
		{
			name: "validate race field values match seed data",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1},
			},
			expectedCount: 3,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				expectedRaces := map[int64]struct {
					name      string
					number    int64
					visible   bool
					meetingID int64
				}{
					1: {name: "Race 1", number: 1, visible: true, meetingID: 1},
					2: {name: "Race 2", number: 2, visible: false, meetingID: 1},
					3: {name: "Race 3", number: 3, visible: true, meetingID: 1},
				}
				for _, race := range races {
					expected, ok := expectedRaces[race.Id]
					if !ok {
						t.Errorf("unexpected race ID: %d", race.Id)
						continue
					}
					if race.Name != expected.name {
						t.Errorf("race %d: expected name %q, got %q", race.Id, expected.name, race.Name)
					}
					if race.Number != expected.number {
						t.Errorf("race %d: expected number %d, got %d", race.Id, expected.number, race.Number)
					}
					if race.Visible != expected.visible {
						t.Errorf("race %d: expected visible %v, got %v", race.Id, expected.visible, race.Visible)
					}
					if race.MeetingId != expected.meetingID {
						t.Errorf("race %d: expected meeting_id %d, got %d", race.Id, expected.meetingID, race.MeetingId)
					}
				}
			},
			seedData: true,
		},
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
	}

	// Default test data seeded for most test cases
	// Includes both past and future times to test time-based scenarios
	now := time.Now()
	defaultTestRaces := []testRace{
		{ID: 1, MeetingID: 1, Name: "Race 1", Number: 1, Visible: true, AdvertisedStartTime: now.Add(-5 * time.Hour)},  // Past
		{ID: 2, MeetingID: 1, Name: "Race 2", Number: 2, Visible: false, AdvertisedStartTime: now.Add(-2 * time.Hour)}, // Past
		{ID: 3, MeetingID: 1, Name: "Race 3", Number: 3, Visible: true, AdvertisedStartTime: now.Add(1 * time.Hour)},   // Future
		{ID: 4, MeetingID: 2, Name: "Race 4", Number: 1, Visible: true, AdvertisedStartTime: now.Add(3 * time.Hour)},   // Future
		{ID: 5, MeetingID: 2, Name: "Race 5", Number: 2, Visible: false, AdvertisedStartTime: now.Add(6 * time.Hour)},  // Future
		{ID: 6, MeetingID: 3, Name: "Race 6", Number: 1, Visible: true, AdvertisedStartTime: now.Add(9 * time.Hour)},   // Future
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			t.Cleanup(cleanup)

			if tc.seedData {
				seedTestData(t, db, defaultTestRaces)
			}

			repo := racingdb.NewRacesRepo(db)
			racingService := service.NewRacingService(repo)

			request := &racingpb.ListRacesRequest{
				Filter: tc.filter,
			}

			response, err := racingService.ListRaces(context.Background(), request)
			if err != nil {
				t.Fatalf("ListRaces failed: %v", err)
			}

			if len(response.Races) != tc.expectedCount {
				t.Errorf("expected %d races, got %d", tc.expectedCount, len(response.Races))
			}

			if tc.validateRaces != nil {
				tc.validateRaces(t, response.Races)
			}
		})
	}
}

// TestListRaces_ErrorHandling tests error scenarios in the service layer
func TestListRaces_ErrorHandling(t *testing.T) {
	testCases := []struct {
		name        string
		setupDB     func(t *testing.T) (*sql.DB, func())
		expectError bool
	}{
		{
			name: "database connection closed",
			setupDB: func(t *testing.T) (*sql.DB, func()) {
				db, cleanup := setupTestDB(t)
				_ = db.Close()
				return db, cleanup
			},
			expectError: true,
		},
		{
			name: "missing races table",
			setupDB: func(t *testing.T) (*sql.DB, func()) {
				db, err := sql.Open("sqlite3", ":memory:")
				if err != nil {
					t.Fatalf("failed to open database: %v", err)
				}
				return db, func() {
					_ = db.Close()
				}
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := tc.setupDB(t)
			t.Cleanup(cleanup)

			repo := racingdb.NewRacesRepo(db)
			racingService := service.NewRacingService(repo)

			request := &racingpb.ListRacesRequest{
				Filter: &racingpb.ListRacesRequestFilter{},
			}

			_, err := racingService.ListRaces(context.Background(), request)
			if tc.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// TestListRaces_ContextCancellation tests that ListRaces respects context cancellation
func TestListRaces_ContextCancellation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	seedTestData(t, db, []testRace{
		{ID: 1, MeetingID: 1, Name: "Race 1", Number: 1, Visible: true, AdvertisedStartTime: time.Now()},
	})

	repo := racingdb.NewRacesRepo(db)
	racingService := service.NewRacingService(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	request := &racingpb.ListRacesRequest{
		Filter: &racingpb.ListRacesRequestFilter{},
	}

	// Use defer/recover to ensure no panic occurs
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ListRaces panicked with cancelled context: %v", r)
		}
	}()

	resp, err := racingService.ListRaces(ctx, request)

	// Assert that either:
	// 1. Context cancellation is respected and returns context.Canceled error, OR
	// 2. Operation completes before context check (SQLite may complete quickly)
	// The important thing is no panic and graceful handling
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			t.Logf("got error with cancelled context: %v", err)
		}
	}

	// If response is returned despite cancellation, ensure it's valid
	if resp != nil {
		if resp.Races == nil {
			t.Error("expected non-nil races slice in response")
		}
	}
}

// TestListRaces_ResponseValidation tests that all response fields are properly populated
func TestListRaces_ResponseValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	now := time.Now()
	seedTestData(t, db, []testRace{
		{ID: 1, MeetingID: 1, Name: "Test Race", Number: 5, Visible: true, AdvertisedStartTime: now},
	})

	repo := racingdb.NewRacesRepo(db)
	racingService := service.NewRacingService(repo)

	request := &racingpb.ListRacesRequest{
		Filter: &racingpb.ListRacesRequestFilter{},
	}

	response, err := racingService.ListRaces(context.Background(), request)
	if err != nil {
		t.Fatalf("ListRaces failed: %v", err)
	}

	if len(response.Races) != 1 {
		t.Fatalf("expected 1 race, got %d", len(response.Races))
	}

	race := response.Races[0]

	// Validate all fields are populated correctly
	if race.Id != 1 {
		t.Fatalf("expected Id 1, got %d", race.Id)
	}
	if race.MeetingId != 1 {
		t.Fatalf("expected MeetingId 1, got %d", race.MeetingId)
	}
	if race.Name != "Test Race" {
		t.Fatalf("expected Name 'Test Race', got %q", race.Name)
	}
	if race.Number != 5 {
		t.Fatalf("expected Number 5, got %d", race.Number)
	}
	if race.Visible != true {
		t.Fatalf("expected Visible true, got %v", race.Visible)
	}
	if race.AdvertisedStartTime == nil {
		t.Fatal("expected AdvertisedStartTime to be set")
	}

	raceTime, err := ptypes.Timestamp(race.AdvertisedStartTime)
	if err != nil {
		t.Fatalf("failed to parse AdvertisedStartTime: %v", err)
	}

	if diff := cmp.Diff(now, raceTime, cmpopts.EquateApproxTime(time.Second)); diff != "" {
		t.Fatalf("AdvertisedStartTime mismatch (-want +got):\n%s", diff)
	}
}

// TestListRaces_Sorting tests the sorting functionality for ListRaces
func TestListRaces_Sorting(t *testing.T) {
	// Create test data with specific times for predictable ordering
	// Mix of past (negative offset), present (zero), and future (positive offset) times
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	sortTestRaces := []testRace{
		{ID: 1, MeetingID: 1, Name: "Zebra Race", Number: 1, Visible: true, AdvertisedStartTime: baseTime.Add(2 * time.Hour)},   // 14:00 (future)
		{ID: 2, MeetingID: 1, Name: "Apple Race", Number: 2, Visible: false, AdvertisedStartTime: baseTime.Add(-1 * time.Hour)}, // 11:00 (past)
		{ID: 3, MeetingID: 2, Name: "Mango Race", Number: 1, Visible: true, AdvertisedStartTime: baseTime.Add(3 * time.Hour)},   // 15:00 (future)
		{ID: 4, MeetingID: 2, Name: "Banana Race", Number: 2, Visible: true, AdvertisedStartTime: baseTime.Add(-2 * time.Hour)}, // 10:00 (past)
		{ID: 5, MeetingID: 3, Name: "Cherry Race", Number: 1, Visible: true, AdvertisedStartTime: baseTime.Add(0 * time.Hour)},  // 12:00 (present)
	}

	testCases := []struct {
		name          string
		filter        *racingpb.ListRacesRequestFilter
		expectedOrder []int64 // Expected race IDs in order
		validateOrder func(t *testing.T, races []*racingpb.Race)
	}{
		{
			name:          "nil filter uses default sort",
			filter:        nil,                    // Entire filter is nil
			expectedOrder: []int64{4, 2, 5, 1, 3}, // 10:00, 11:00, 12:00, 14:00, 15:00
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				// Verify ascending order (earliest first)
				for i := 1; i < len(races); i++ {
					prevTime, _ := ptypes.Timestamp(races[i-1].AdvertisedStartTime)
					currTime, _ := ptypes.Timestamp(races[i].AdvertisedStartTime)
					if currTime.Before(prevTime) {
						t.Errorf("races not in ascending order: race %d (time: %v) before race %d (time: %v)",
							races[i].Id, currTime, races[i-1].Id, prevTime)
					}
				}
			},
		},
		{
			name:          "nil sort_by uses default advertised_start_time ascending",
			filter:        &racingpb.ListRacesRequestFilter{}, // SortBy is nil (omitted)
			expectedOrder: []int64{4, 2, 5, 1, 3},             // 10:00, 11:00, 12:00, 14:00, 15:00
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				// Verify ascending order (earliest first)
				for i := 1; i < len(races); i++ {
					prevTime, _ := ptypes.Timestamp(races[i-1].AdvertisedStartTime)
					currTime, _ := ptypes.Timestamp(races[i].AdvertisedStartTime)
					if currTime.Before(prevTime) {
						t.Errorf("races not in ascending order: race %d (time: %v) before race %d (time: %v)",
							races[i].Id, currTime, races[i-1].Id, prevTime)
					}
				}
			},
		},
		{
			name: "explicit sort by advertised_start_time ascending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("advertised_start_time"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{4, 2, 5, 1, 3},
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				expectedIDs := []int64{4, 2, 5, 1, 3}
				for i, race := range races {
					if race.Id != expectedIDs[i] {
						t.Errorf("position %d: expected race %d, got race %d", i, expectedIDs[i], race.Id)
					}
				}
			},
		},
		{
			name: "sort by advertised_start_time descending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("advertised_start_time"),
				Descending: proto.Bool(true),
			},
			expectedOrder: []int64{3, 1, 5, 2, 4}, // 15:00, 14:00, 12:00, 11:00, 10:00
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				expectedIDs := []int64{3, 1, 5, 2, 4}
				for i, race := range races {
					if race.Id != expectedIDs[i] {
						t.Errorf("position %d: expected race %d, got race %d", i, expectedIDs[i], race.Id)
					}
				}
			},
		},
		{
			name: "sort by name ascending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("name"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{2, 4, 5, 3, 1}, // Apple, Banana, Cherry, Mango, Zebra
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				expectedNames := []string{"Apple Race", "Banana Race", "Cherry Race", "Mango Race", "Zebra Race"}
				for i, race := range races {
					if race.Name != expectedNames[i] {
						t.Errorf("position %d: expected name %q, got %q", i, expectedNames[i], race.Name)
					}
				}
			},
		},
		{
			name: "sort by name descending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("name"),
				Descending: proto.Bool(true),
			},
			expectedOrder: []int64{1, 3, 5, 4, 2}, // Zebra, Mango, Cherry, Banana, Apple
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				expectedNames := []string{"Zebra Race", "Mango Race", "Cherry Race", "Banana Race", "Apple Race"}
				for i, race := range races {
					if race.Name != expectedNames[i] {
						t.Errorf("position %d: expected name %q, got %q", i, expectedNames[i], race.Name)
					}
				}
			},
		},
		{
			name: "sort by id ascending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("id"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{1, 2, 3, 4, 5},
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				expectedIDs := []int64{1, 2, 3, 4, 5}
				for i, race := range races {
					if race.Id != expectedIDs[i] {
						t.Errorf("position %d: expected race %d, got race %d", i, expectedIDs[i], race.Id)
					}
				}
			},
		},
		{
			name: "sort by id descending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("id"),
				Descending: proto.Bool(true),
			},
			expectedOrder: []int64{5, 4, 3, 2, 1},
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				expectedIDs := []int64{5, 4, 3, 2, 1}
				for i, race := range races {
					if race.Id != expectedIDs[i] {
						t.Errorf("position %d: expected race %d, got race %d", i, expectedIDs[i], race.Id)
					}
				}
			},
		},
		{
			name: "sort by meeting_id ascending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("meeting_id"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{1, 2, 3, 4, 5}, // Meeting 1 (races 1,2), Meeting 2 (races 3,4), Meeting 3 (race 5)
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				// All meeting 1 races should come before meeting 2 races, which come before meeting 3
				lastMeetingID := int64(0)
				for _, race := range races {
					if race.MeetingId < lastMeetingID {
						t.Errorf("meeting_id out of order: got %d after %d", race.MeetingId, lastMeetingID)
					}
					lastMeetingID = race.MeetingId
				}
			},
		},
		{
			name: "sort by number ascending",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("number"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{1, 3, 5, 2, 4}, // Number 1 (races 1,3,5), Number 2 (races 2,4)
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				// All number 1 races should come before number 2 races
				foundNumber2 := false
				for _, race := range races {
					if race.Number == 2 {
						foundNumber2 = true
					}
					if race.Number == 1 && foundNumber2 {
						t.Error("number 1 race found after number 2 race")
					}
				}
			},
		},
		{
			name: "empty string sort_by uses default advertised_start_time",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String(""), // Explicitly set to empty string
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{4, 2, 5, 1, 3}, // Should fallback to default sort
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				// Verify sorted by time (default)
				for i := 1; i < len(races); i++ {
					prevTime, _ := ptypes.Timestamp(races[i-1].AdvertisedStartTime)
					currTime, _ := ptypes.Timestamp(races[i].AdvertisedStartTime)
					if currTime.Before(prevTime) {
						t.Errorf("empty sort_by did not fallback to default order")
					}
				}
			},
		},
		{
			name: "invalid sort_by falls back to advertised_start_time",
			filter: &racingpb.ListRacesRequestFilter{
				SortBy:     proto.String("invalid_field"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{4, 2, 5, 1, 3}, // Should fallback to default sort
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 5 {
					t.Fatalf("expected 5 races, got %d", len(races))
				}
				// Verify sorted by time (default)
				for i := 1; i < len(races); i++ {
					prevTime, _ := ptypes.Timestamp(races[i-1].AdvertisedStartTime)
					currTime, _ := ptypes.Timestamp(races[i].AdvertisedStartTime)
					if currTime.Before(prevTime) {
						t.Errorf("races not in default order: race %d before race %d", races[i].Id, races[i-1].Id)
					}
				}
			},
		},
		{
			name: "combined filter and sort",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1},
				SortBy:     proto.String("advertised_start_time"),
				Descending: proto.Bool(true),
			},
			expectedOrder: []int64{1, 2}, // Meeting 1 races: 14:00, 11:00 (descending)
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				if len(races) != 2 {
					t.Fatalf("expected 2 races, got %d", len(races))
				}
				for _, race := range races {
					if race.MeetingId != 1 {
						t.Errorf("expected meeting_id 1, got %d", race.MeetingId)
					}
				}
				// Verify descending order
				firstTime, _ := ptypes.Timestamp(races[0].AdvertisedStartTime)
				secondTime, _ := ptypes.Timestamp(races[1].AdvertisedStartTime)
				if !firstTime.After(secondTime) {
					t.Error("expected descending order")
				}
			},
		},
		{
			name: "sort with visible filter",
			filter: &racingpb.ListRacesRequestFilter{
				Visible:    proto.Bool(true),
				SortBy:     proto.String("advertised_start_time"),
				Descending: proto.Bool(false),
			},
			expectedOrder: []int64{5, 1, 3}, // Visible races: 12:00, 14:00, 15:00
			validateOrder: func(t *testing.T, races []*racingpb.Race) {
				// Verify all races are visible
				for _, race := range races {
					if !race.Visible {
						t.Errorf("expected only visible races, got race %d with visible=%v",
							race.Id, race.Visible)
					}
				}
				// Verify ascending order
				for i := 1; i < len(races); i++ {
					prevTime, _ := ptypes.Timestamp(races[i-1].AdvertisedStartTime)
					currTime, _ := ptypes.Timestamp(races[i].AdvertisedStartTime)
					if currTime.Before(prevTime) {
						t.Error("visible filtered races not in ascending order")
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			t.Cleanup(cleanup)

			seedTestData(t, db, sortTestRaces)

			repo := racingdb.NewRacesRepo(db)
			racingService := service.NewRacingService(repo)

			request := &racingpb.ListRacesRequest{
				Filter: tc.filter,
			}

			response, err := racingService.ListRaces(context.Background(), request)
			if err != nil {
				t.Fatalf("ListRaces failed: %v", err)
			}

			if tc.validateOrder != nil {
				tc.validateOrder(t, response.Races)
			}
		})
	}
}
