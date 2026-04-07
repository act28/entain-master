package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"

	"git.neds.sh/matty/entain/racing/proto/racing"
)

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

type racesRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewRacesRepo creates a new races repository.
func NewRacesRepo(db *sql.DB) RacesRepo {
	return &racesRepo{db: db}
}

// Init prepares the race repository dummy data.
func (r *racesRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy races.
		err = r.seed()
	})

	return err
}

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
//
// SECURITY NOTE: This function uses fmt.Sprintf to construct the ORDER BY clause.
// The sortBy and direction values are safe because:
// 1. sortBy is validated against a whitelist in validateSortField() - only known column names are allowed
// 2. direction is strictly controlled to only "ASC" or "DESC" constants
// This prevents SQL injection while allowing dynamic sorting.
func (r *racesRepo) applySorting(query string, filter *racing.ListRacesRequestFilter) string {
	// Determine sort field (default to advertised_start_time)
	// validateSortField ensures only whitelisted column names are used
	sortBy := "advertised_start_time"
	if filter != nil && filter.SortBy != nil && *filter.SortBy != "" {
		sortBy = r.validateSortField(*filter.SortBy)
	}

	// Determine sort direction (default ASC, only ASC or DESC allowed)
	direction := "ASC"
	if filter != nil && filter.Descending != nil && *filter.Descending {
		direction = "DESC"
	}

	// Safe to use fmt.Sprintf because sortBy and direction are controlled values
	return query + fmt.Sprintf(" ORDER BY %s %s", sortBy, direction)
}

// validateSortField ensures the sort field is valid to prevent SQL injection.
// Returns the field name if valid, otherwise returns default "advertised_start_time".
func (r *racesRepo) validateSortField(field string) string {
	switch field {
	case "advertised_start_time", "name", "id", "meeting_id", "number":
		return field
	}
	// Return safe default if invalid field provided
	return "advertised_start_time"
}

// computeRaceStatus determines if a race is OPEN or CLOSED based on the
// advertised start time compared to the current time.
//
// A race is:
// - OPEN if the current time is before the advertised start time
// - CLOSED if the current time is at or after the advertised start time
func computeRaceStatus(advertisedStart time.Time) racing.Race_Status {
	// Ensure consistent timezone handling by using UTC for comparison
	if time.Now().UTC().Before(advertisedStart.UTC()) {
		return racing.Race_OPEN
	}
	return racing.Race_CLOSED
}

func (m *racesRepo) scanRaces(
	rows *sql.Rows,
) ([]*racing.Race, error) {
	var races []*racing.Race

	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
			return nil, fmt.Errorf("failed to scan race row: %w", err)
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, fmt.Errorf("failed to convert timestamp: %w", err)
		}

		race.AdvertisedStartTime = ts

		// Compute status based on advertised_start_time vs current time
		race.Status = computeRaceStatus(advertisedStart)

		races = append(races, &race)
	}

	return races, nil
}

// Get returns a single race by its ID.
// Returns an error if the race is not found or if a database error occurs.
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("race with id %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan race: %w", err)
	}

	ts, err := ptypes.TimestampProto(advertisedStart)
	if err != nil {
		return nil, fmt.Errorf("failed to convert timestamp: %w", err)
	}

	race.AdvertisedStartTime = ts

	// Compute status based on advertised_start_time vs current time
	race.Status = computeRaceStatus(advertisedStart)

	return &race, nil
}
