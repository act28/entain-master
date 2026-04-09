package tests

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// testEvent represents a test event entry for seeding the database.
type testEvent struct {
	ID                  int64
	SportID             int64
	Name                string
	Visible             bool
	AdvertisedStartTime time.Time
}

// setupTestDB creates a test database with sport_types and events tables.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	ctx := context.Background()

	// Create in-memory SQLite db for testing.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create sport_types table first (for referential integrity).
	_, err = db.ExecContext(ctx, `
		CREATE TABLE sport_types (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create sport_types table: %v", err)
	}

	// Seed sport_types for tests.
	_, err = db.ExecContext(ctx, `
		INSERT INTO sport_types (id, name) VALUES
			(1, 'Football'),
			(2, 'Basketball'),
			(3, 'Tennis')
	`)
	if err != nil {
		t.Fatalf("failed to seed sport_types: %v", err)
	}

	// Create events table with foreign key.
	_, err = db.ExecContext(ctx, `
		CREATE TABLE events (
			id INTEGER PRIMARY KEY,
			sport_id INTEGER NOT NULL,
			name TEXT,
			visible BOOLEAN,
			advertised_start_time TIMESTAMP,
			FOREIGN KEY (sport_id) REFERENCES sport_types(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create events table: %v", err)
	}

	return db, func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database: %v", err)
		}
	}
}

// seedTestData inserts test event data into the database.
func seedTestData(t *testing.T, db *sql.DB, testEvents []testEvent) {
	t.Helper()

	if len(testEvents) == 0 {
		return
	}

	ctx := context.Background()

	// Insert each event using parameterized query.
	for _, e := range testEvents {
		_, err := db.ExecContext(
			ctx,
			`INSERT INTO events (id, sport_id, name, visible, advertised_start_time)
			 VALUES (?, ?, ?, ?, ?)`,
			e.ID, e.SportID, e.Name, e.Visible, e.AdvertisedStartTime,
		)
		if err != nil {
			t.Fatalf("failed to insert test event: %v", err)
		}
	}
}
