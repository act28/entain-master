package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"syreclabs.com/go/faker"
)

const maxDataSeedItems = 100

var sportTypes = []string{
	"Football",
	"Basketball",
	"Tennis",
	"Baseball",
	"Hockey",
}

// createSchema creates the database schema (tables and indexes).
// This should be called before seeding data.
func (r *eventsRepo) createSchema(ctx context.Context) error {
	// Create tables.
	if _, err := r.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS sport_types (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL
        )
    `); err != nil {
		return fmt.Errorf("failed to create sport_types table: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY,
            sport_id INTEGER,
            name TEXT,
            visible INTEGER,
            advertised_start_time DATETIME
        )
    `); err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	// Create indexes for query optimization.
	// Separate indexes for flexibility - support filtering by sport_id and sorting by advertised_start_time independently.
	if _, err := r.db.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_events_sport_id
        ON events(sport_id)
    `); err != nil {
		return fmt.Errorf("failed to create sport_id index: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_events_start_time
        ON events(advertised_start_time)
    `); err != nil {
		return fmt.Errorf("failed to create start_time index: %w", err)
	}

	return nil
}

// seed populates the database with initial/dummy data.
// Tables must already exist before calling this function.
func (r *eventsRepo) seed(ctx context.Context) error {
	// Wrap everything in a transaction.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Rollback on error, Commit on success.
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Prepare statements once outside loops.
	insertSportType, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO sport_types(id, name) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare sport_type insert: %w", err)
	}
	defer func() { _ = insertSportType.Close() }()

	insertEvent, err := tx.PrepareContext(ctx, `
        INSERT OR IGNORE INTO events(id, sport_id, name, visible, advertised_start_time)
        VALUES (?,?,?,?,?)
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare event insert: %w", err)
	}
	defer func() { _ = insertEvent.Close() }()

	// Seed sport types.
	for i, name := range sportTypes {
		if _, err = insertSportType.ExecContext(ctx, i+1, name); err != nil {
			return fmt.Errorf("failed to insert sport_type %d: %w", i+1, err)
		}
	}

	// Seed events.
	for i := 1; i <= maxDataSeedItems; i++ {
		if _, err = insertEvent.ExecContext(
			ctx,
			i,
			faker.Number().Between(1, 5), //nolint:mnd // ok for test seed data.
			faker.Team().Name()+" vs "+faker.Team().Name(),
			faker.Number().Between(0, 1),
			faker.Time().Between(
				time.Now().AddDate(0, 0, -1),
				time.Now().AddDate(0, 0, 2),
			).Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("failed to insert event %d: %w", i, err)
		}
	}

	return nil
}
