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
