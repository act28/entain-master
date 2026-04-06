package tests

import (
	"database/sql"
	"testing"
	"time"
)

// testRace represents a test race entry for seeding the database
type testRace struct {
	ID                  int64
	MeetingID           int64
	Name                string
	Number              int64
	Visible             bool
	AdvertisedStartTime time.Time // Absolute time for advertised_start_time
}

// setupTestDB creates a test database and returns the db connection and cleanup function
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	// Create in-memory SQLite db for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create races table
	_, err = db.Exec(`
		CREATE TABLE races (
			id INTEGER PRIMARY KEY,
			meeting_id INTEGER,
			name TEXT,
			number INTEGER,
			visible BOOLEAN,
			advertised_start_time TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create races table: %v", err)
	}

	return db, func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database: %v", err)
		}
	}
}

// seedTestData inserts test race data into the database using parameterized queries.
func seedTestData(t *testing.T, db *sql.DB, testRaces []testRace) {
	t.Helper()

	if len(testRaces) == 0 {
		return
	}

	// Insert each race using parameterized query
	for _, r := range testRaces {
		_, err := db.Exec(
			`INSERT INTO races (id, meeting_id, name, number, visible, advertised_start_time)
				 VALUES (?, ?, ?, ?, ?, ?)`,
			r.ID, r.MeetingID, r.Name, r.Number, r.Visible, r.AdvertisedStartTime,
		)
		if err != nil {
			t.Fatalf("failed to insert test race: %v", err)
		}
	}
}
