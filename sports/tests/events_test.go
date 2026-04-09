package tests

import (
	"context"
	"testing"
	"time"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"git.neds.sh/matty/entain/sports/service"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

// TestEventsRepo_ListEvents_Service tests the List method with various filter scenarios.
func TestEventsRepo_ListEvents_Service(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	// Default test events.
	defaultTestEvents := []testEvent{
		{ID: 1, SportID: 1, Name: "Football Match A", Visible: true, AdvertisedStartTime: now.Add(2 * time.Hour)},
		{ID: 2, SportID: 1, Name: "Football Match B", Visible: false, AdvertisedStartTime: now.Add(4 * time.Hour)},
		{ID: 3, SportID: 2, Name: "Basketball Game", Visible: true, AdvertisedStartTime: now.Add(-1 * time.Hour)}, // CLOSED
		{ID: 4, SportID: 3, Name: "Tennis Match", Visible: true, AdvertisedStartTime: now.Add(6 * time.Hour)},
	}

	tests := []struct {
		name       string
		filter     *sports.ListEventsRequestFilter
		wantIDs    []int64
		wantErr    bool
		seedEvents []testEvent
	}{
		{
			name:       "nil filter returns all events",
			filter:     nil,
			wantIDs:    []int64{3, 1, 2, 4}, // sorted by advertised_start_time ASC: -1hr, +2hr, +4hr, +6hr
			seedEvents: defaultTestEvents,
		},
		{
			name:       "empty filter returns all events",
			filter:     &sports.ListEventsRequestFilter{},
			wantIDs:    []int64{3, 1, 2, 4}, // sorted by advertised_start_time ASC: -1hr, +2hr, +4hr, +6hr
			seedEvents: defaultTestEvents,
		},
		{
			name:       "empty database returns empty list",
			filter:     &sports.ListEventsRequestFilter{},
			wantIDs:    []int64{},
			seedEvents: []testEvent{},
		},
		{
			name: "filter by single sport_id",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{1},
			},
			wantIDs:    []int64{1, 2},
			seedEvents: defaultTestEvents,
		},
		{
			name: "filter by multiple sport_ids",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{1, 2},
			},
			wantIDs:    []int64{3, 1, 2}, // sorted by advertised_start_time ASC: -1hr (basketball), +2hr, +4hr (football)
			seedEvents: defaultTestEvents,
		},
		{
			name: "filter by visible=true",
			filter: &sports.ListEventsRequestFilter{
				Visible: proto.Bool(true),
			},
			wantIDs:    []int64{3, 1, 4}, // sorted by advertised_start_time ASC: -1hr, +2hr, +6hr
			seedEvents: defaultTestEvents,
		},
		{
			name: "filter by visible=false",
			filter: &sports.ListEventsRequestFilter{
				Visible: proto.Bool(false),
			},
			wantIDs:    []int64{2},
			seedEvents: defaultTestEvents,
		},
		{
			name: "non-existent sport_ids returns empty",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{99, 100},
			},
			wantIDs:    []int64{},
			seedEvents: defaultTestEvents,
		},
		{
			name: "combined filters - sport_ids + visible",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{1},
				Visible:  proto.Bool(true),
			},
			wantIDs:    []int64{1},
			seedEvents: defaultTestEvents,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test database.
			testDB, cleanup := setupTestDB(t)
			defer cleanup()

			// Seed test data.
			seedTestData(t, testDB, tt.seedEvents)

			// Create repository.
			repo := db.NewEventsRepo(testDB)

			// Call List.
			got, err := repo.List(context.Background(), tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Extract IDs from result for comparison.
			gotIDs := make([]int64, len(got))
			for i, event := range got {
				gotIDs[i] = event.Id
			}

			// Compare IDs.
			if diff := cmp.Diff(tt.wantIDs, gotIDs); diff != "" {
				t.Errorf("List() IDs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEventsRepo_ListEvents_Sorting tests the List method with various sorting options.
func TestEventsRepo_ListEvents_Sorting(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	// Test events with different times for sorting.
	testEvents := []testEvent{
		{ID: 1, SportID: 1, Name: "Alpha Match", Visible: true, AdvertisedStartTime: now.Add(4 * time.Hour)},
		{ID: 2, SportID: 1, Name: "Beta Match", Visible: true, AdvertisedStartTime: now.Add(2 * time.Hour)},
		{ID: 3, SportID: 2, Name: "Gamma Game", Visible: true, AdvertisedStartTime: now.Add(6 * time.Hour)},
	}

	tests := []struct {
		name       string
		sortBy     string
		descending bool
		wantIDs    []int64
	}{
		{
			name:       "sort by advertised_start_time ASC (default)",
			sortBy:     "",
			descending: false,
			wantIDs:    []int64{2, 1, 3}, // earliest first
		},
		{
			name:       "sort by advertised_start_time DESC",
			sortBy:     "advertised_start_time",
			descending: true,
			wantIDs:    []int64{3, 1, 2}, // latest first
		},
		{
			name:       "sort by name ASC",
			sortBy:     "name",
			descending: false,
			wantIDs:    []int64{1, 2, 3}, // Alpha, Beta, Gamma
		},
		{
			name:       "sort by name DESC",
			sortBy:     "name",
			descending: true,
			wantIDs:    []int64{3, 2, 1}, // Gamma, Beta, Alpha
		},
		{
			name:       "sort by id ASC",
			sortBy:     "id",
			descending: false,
			wantIDs:    []int64{1, 2, 3},
		},
		{
			name:       "sort by id DESC",
			sortBy:     "id",
			descending: true,
			wantIDs:    []int64{3, 2, 1},
		},
		{
			name:       "sort by sport_id ASC",
			sortBy:     "sport_id",
			descending: false,
			wantIDs:    []int64{1, 2, 3}, // 1, 1, 2
		},
		{
			name:       "invalid sort_by defaults to advertised_start_time",
			sortBy:     "invalid_field",
			descending: false,
			wantIDs:    []int64{2, 1, 3}, // same as default
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test database.
			testDB, cleanup := setupTestDB(t)
			defer cleanup()

			// Seed test data.
			seedTestData(t, testDB, testEvents)

			// Create repository.
			repo := db.NewEventsRepo(testDB)

			// Build filter.
			filter := &sports.ListEventsRequestFilter{
				SortBy:     &tt.sortBy,
				Descending: &tt.descending,
			}

			// Call List.
			got, err := repo.List(context.Background(), filter)
			if err != nil {
				t.Errorf("List() error = %v", err)
				return
			}

			// Extract IDs from result.
			gotIDs := make([]int64, len(got))
			for i, event := range got {
				gotIDs[i] = event.Id
			}

			// Compare IDs.
			if diff := cmp.Diff(tt.wantIDs, gotIDs); diff != "" {
				t.Errorf("List() IDs order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEventsRepo_ListEvents_StatusValidation tests that status is computed correctly.
func TestEventsRepo_ListEvents_StatusValidation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	// Test events with specific times for status validation.
	testEvents := []testEvent{
		{ID: 1, SportID: 1, Name: "Future Event", Visible: true, AdvertisedStartTime: now.Add(2 * time.Hour)},
		{ID: 2, SportID: 1, Name: "Past Event", Visible: true, AdvertisedStartTime: now.Add(-2 * time.Hour)},
	}

	tests := []struct {
		name       string
		eventID    int64
		wantStatus sports.Event_Status
	}{
		{
			name:       "future event has OPEN status",
			eventID:    1,
			wantStatus: sports.Event_OPEN,
		},
		{
			name:       "past event has CLOSED status",
			eventID:    2,
			wantStatus: sports.Event_CLOSED,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test database.
			testDB, cleanup := setupTestDB(t)
			defer cleanup()

			// Seed test data.
			seedTestData(t, testDB, testEvents)

			// Create repository.
			repo := db.NewEventsRepo(testDB)

			// Call List to get events and check status.
			filter := &sports.ListEventsRequestFilter{
				SportIds: []int64{tt.eventID},
			}
			got, err := repo.List(context.Background(), filter)
			if err != nil {
				t.Errorf("List() error = %v", err)
				return
			}

			if len(got) == 0 {
				t.Errorf("List() expected event with id %d but got none", tt.eventID)
				return
			}

			if got[0].Status != tt.wantStatus {
				t.Errorf("List() Status = %v, want %v", got[0].Status, tt.wantStatus)
			}
		})
	}
}

// TestEventsService_ListEvents_FilterValidation tests the service layer filter validation.
func TestEventsService_ListEvents_FilterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filter  *sports.ListEventsRequestFilter
		wantErr bool
	}{
		{
			name:    "nil filter is valid",
			filter:  nil,
			wantErr: false,
		},
		{
			name: "empty filter is valid",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{},
			},
			wantErr: false,
		},
		{
			name: "valid sport_ids",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{1, 2, 3},
			},
			wantErr: false,
		},
		{
			name: "invalid sport_id - zero",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{0, 1},
			},
			wantErr: true,
		},
		{
			name: "invalid sport_id - negative",
			filter: &sports.ListEventsRequestFilter{
				SportIds: []int64{-1, 1},
			},
			wantErr: true,
		},
		{
			name: "sort by valid field",
			filter: &sports.ListEventsRequestFilter{
				SortBy: proto.String("name"),
			},
			wantErr: false,
		},
		{
			name: "sort by invalid field defaults to advertised_start_time",
			filter: &sports.ListEventsRequestFilter{
				SortBy: proto.String("invalid_field"),
			},
			wantErr: false, // Invalid sort_by fields are handled safely by defaulting to advertised_start_time
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test database.
			testDB, cleanup := setupTestDB(t)
			defer cleanup()

			// Seed minimal test data.
			seedTestData(t, testDB, []testEvent{
				{ID: 1, SportID: 1, Name: "Test Event", Visible: true, AdvertisedStartTime: time.Now().Add(time.Hour)},
			})

			// Create repository and service.
			repo := db.NewEventsRepo(testDB)
			svc := service.NewSportsService(repo)

			// Call ListEvents.
			ctx := context.Background()
			req := &sports.ListEventsRequest{Filter: tt.filter}
			_, err := svc.ListEvents(ctx, req)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListEvents() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
