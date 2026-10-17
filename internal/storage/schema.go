package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql baseline.sql
var migrations embed.FS

const schemaVersion = 8
const baselineVersion = 8

func migrate(ctx context.Context, db *sql.DB, scripts []string, baseline string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migration: acquire database: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("storage migration bookkeeping: %w", err)
	}
	var version, count int
	if err = tx.QueryRowContext(ctx, `SELECT coalesce(max(version),0),count(*) FROM schema_migrations`).Scan(&version, &count); err != nil {
		return err
	}
	if version > len(scripts) {
		return fmt.Errorf("storage schema %d is newer than supported %d; upgrade the application", version, len(scripts))
	}
	if count != version {
		return fmt.Errorf("storage migration history is not contiguous")
	}
	if version == 0 && baseline != "" {
		if baselineVersion > len(scripts) {
			return fmt.Errorf("storage baseline is newer than supported migrations")
		}
		if _, err = tx.ExecContext(ctx, baseline); err != nil {
			return fmt.Errorf("storage fresh schema failed; no initialization committed: %w", err)
		}
		// Mark the historical versions satisfied by the snapshot, without
		// executing their obsolete table creation, rebuilds or data updates.
		for i := 1; i <= baselineVersion; i++ {
			if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (?)`, i); err != nil {
				return err
			}
		}
		version = baselineVersion
	}
	for i := version; i < len(scripts); i++ {
		if _, err = tx.ExecContext(ctx, scripts[i]); err != nil {
			return fmt.Errorf("storage migration %d failed; no upgrade committed: %w", i+1, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (?)`, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) initialize(ctx context.Context) error {
	scripts := []string{}
	for _, name := range []string{"001_initial.sql", "002_discovery.sql", "003_growatt.sql", "004_growatt_discovery_failure.sql", "005_aggregates.sql", "006_schedules.sql", "007_continuous_history.sql", "008_canonical_readings.sql"} {
		script, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		scripts = append(scripts, string(script))
	}
	baseline, err := migrations.ReadFile("baseline.sql")
	if err != nil {
		return err
	}
	if err := migrate(ctx, s.db, scripts, string(baseline)); err != nil {
		return err
	}
	_, err = s.RecoverInterruptedAttempts(ctx, At(s.now()))
	return err
}
