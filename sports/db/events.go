package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/types/known/timestamppb"

	"git.neds.sh/matty/entain/sports/proto/sports"
)

const defaultSortField = "advertised_start_time"

// sortFieldMap maps user-friendly field names to safe database column names.
var sortFieldMap = map[string]string{
	defaultSortField: "e." + defaultSortField,
	"name":           "e.name",
	"id":             "e.id",
	"sport_id":       "e.sport_id",
}

// EventsRepo provides repository access to events.
type EventsRepo interface {
	// Init will initialise our events repository.
	Init(ctx context.Context) error

	// List will return a list of events.
	List(ctx context.Context, filter *sports.ListEventsRequestFilter) ([]*sports.Event, error)
}

type eventsRepo struct {
	db   *sql.DB
	init sync.Once
	// initError stores any error that occurred during initialization.
	// This allows the sync.Once to complete while still capturing
	// the error for callers to handle on subsequent calls to Init().
	initError error
}

// NewEventsRepo creates a new events repository.
func NewEventsRepo(db *sql.DB) EventsRepo {
	return &eventsRepo{db: db}
}

// Init prepares the event repository dummy data.
// This method uses sync.Once to ensure initialization happens only once.
// If initialization fails, the error is returned and the repository is in
// an unusable state. Callers should treat initialization failure as a fatal
// error and exit the application (fail-fast principle). Subsequent calls
// after a failure will return the same error without retry.
func (r *eventsRepo) Init(ctx context.Context) error {
	r.init.Do(func() {
		// Create schema first (tables and indexes).
		if err := r.createSchema(ctx); err != nil {
			r.initError = err
			return
		}
		// Then seed with data.
		r.initError = r.seed(ctx)
	})

	return r.initError
}

func (r *eventsRepo) List(ctx context.Context, filter *sports.ListEventsRequestFilter) ([]*sports.Event, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getEventQueries()[eventsList]

	// Step 1: Apply WHERE clause (filtering).
	query, args = r.applyFilter(query, filter)

	// Step 2: Apply ORDER BY (sorting).
	// Separate from filtering for single responsibility.
	query = r.applySorting(query, filter)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list query error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return r.scanEvents(ctx, rows)
}

// buildInClause creates an IN clause with placeholders for SQL query.
// Returns the clause string like "sport_id IN (?,?,?)" and the number of placeholders.
func buildInClause(field string, count int) string {
	if count <= 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return fmt.Sprintf("%s IN (%s)", field, strings.Join(placeholders, ","))
}

func (r *eventsRepo) applyFilter(query string, filter *sports.ListEventsRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.SportIds) > 0 {
		sportIDs := filter.SportIds
		clauses = append(clauses, buildInClause("e.sport_id", len(sportIDs)))

		for _, sportID := range sportIDs {
			args = append(args, sportID)
		}
	}

	// Add visible filter if specified.
	if filter.Visible != nil {
		clauses = append(clauses, "e.visible = ?")
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
// SECURITY: This function uses string concatenation for the ORDER BY clause.
// The sortBy value is safe because validateSortField() performs a whitelist lookup
// that only returns known column names. The direction is strictly "ASC" or "DESC".
// This approach prevents SQL injection while allowing dynamic sorting.
func (r *eventsRepo) applySorting(query string, filter *sports.ListEventsRequestFilter) string {
	// Determine sort field (default to advertised_start_time).
	// validateSortField performs a whitelist lookup for safety.
	sortBy := sortFieldMap[defaultSortField]
	if filter != nil && filter.SortBy != nil && *filter.SortBy != "" {
		sortBy = r.validateSortField(*filter.SortBy)
	}

	// Determine sort direction (default ASC, only ASC or DESC allowed).
	direction := "ASC"
	if filter != nil && filter.Descending != nil && *filter.Descending {
		direction = "DESC"
	}

	// Safe to concatenate because sortBy is whitelisted and direction is controlled.
	return query + " ORDER BY " + sortBy + " " + direction
}

// validateSortField performs a whitelist lookup to ensure the sort field is valid.
// This prevents SQL injection by only allowing known column names.
// Returns the safe column name if valid, otherwise returns default "e.advertised_start_time".
func (r *eventsRepo) validateSortField(field string) string {
	if col, ok := sortFieldMap[field]; ok {
		return col
	}
	// Return safe default if invalid field provided.
	return sortFieldMap[defaultSortField]
}

// computeEventStatus determines if an event is OPEN or CLOSED based on the
// advertised start time compared to the current time.
//
// An event is:
// - OPEN if the current time is before the advertised start time.
// - CLOSED if the current time is at or after the advertised start time.
func computeEventStatus(advertisedStart time.Time) sports.Event_Status {
	// Ensure consistent timezone handling by using UTC for comparison.
	if time.Now().UTC().Before(advertisedStart.UTC()) {
		return sports.Event_OPEN
	}
	return sports.Event_CLOSED
}

// defaultEventCapacity is the initial capacity for the events slice.
const defaultEventCapacity = 100

func (r *eventsRepo) scanEvents(
	ctx context.Context,
	rows *sql.Rows,
) ([]*sports.Event, error) {
	events := make([]*sports.Event, 0, defaultEventCapacity)

	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err() //nolint:wrapcheck // could be nil.
		default:
			var event sports.Event
			var advertisedStart time.Time

			if err := rows.Scan(&event.Id, &event.SportId, &event.SportTypeName, &event.Name, &event.Visible, &advertisedStart); err != nil {
				return nil, fmt.Errorf("failed to scan event row: %w", err)
			}

			event.AdvertisedStartTime = timestamppb.New(advertisedStart)

			// Compute status based on advertised_start_time vs current time.
			event.Status = computeEventStatus(advertisedStart)

			events = append(events, &event)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating event rows: %w", err)
		}
	}

	return events, nil
}
