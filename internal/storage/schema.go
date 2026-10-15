package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrations embed.FS

const schemaVersion = 4

func migrate(ctx context.Context, db *sql.DB, scripts []string) error {
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
	for _, name := range []string{"001_initial.sql", "002_discovery.sql", "003_growatt.sql", "004_growatt_discovery_failure.sql"} {
		script, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		scripts = append(scripts, string(script))
	}
	if err := migrate(ctx, s.db, scripts); err != nil {
		return err
	}
	_, err := s.RecoverInterruptedAttempts(ctx, At(s.now()))
	return err
}
