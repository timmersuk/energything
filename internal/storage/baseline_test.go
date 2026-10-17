package storage

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

func schemaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	return db
}

func legacyScripts(t *testing.T) []string {
	t.Helper()
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	scripts := []string{}
	for _, entry := range entries {
		b, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		scripts = append(scripts, string(b))
	}
	return scripts
}

func schemaDefinition(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT type||':'||name||':'||coalesce(sql,'') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFreshBaselineMatchesUpgradeWithoutExecutingLegacyChanges(t *testing.T) {
	ctx := context.Background()
	upgraded := schemaTestDB(t)
	scripts := legacyScripts(t)
	if err := migrate(ctx, upgraded, scripts, ""); err != nil {
		t.Fatal(err)
	}
	baseline, err := migrations.ReadFile("baseline.sql")
	if err != nil {
		t.Fatal(err)
	}
	fresh := schemaTestDB(t)
	// Any attempt to replay a development migration makes this test fail.
	poisoned := append([]string{}, scripts...)
	for i := 0; i < baselineVersion; i++ {
		poisoned[i] = "invalid historical SQL"
	}
	if err = migrate(ctx, fresh, poisoned, string(baseline)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(schemaDefinition(t, fresh), schemaDefinition(t, upgraded)) {
		t.Fatal("fresh and upgraded schemas diverge")
	}
	// Reopening an existing database must not execute its baseline again.
	if err = migrate(ctx, fresh, scripts, "invalid baseline SQL"); err != nil {
		t.Fatal(err)
	}
	// Only changes after the baseline are executed on a first installation.
	withFuture := append(poisoned, `CREATE TABLE synthetic_future(value TEXT)`)
	future := schemaTestDB(t)
	if err = migrate(ctx, future, withFuture, string(baseline)); err != nil {
		t.Fatal(err)
	}
	var created int
	if err = future.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='synthetic_future'`).Scan(&created); err != nil || created != 1 {
		t.Fatal("post-baseline upgrade was skipped", created, err)
	}
}

func TestFreshBaselineFailureIsAtomic(t *testing.T) {
	db := schemaTestDB(t)
	scripts := legacyScripts(t)
	if err := migrate(context.Background(), db, scripts, `CREATE TABLE partial_install(value TEXT); invalid SQL`); err == nil {
		t.Fatal("broken baseline accepted")
	}
	if len(schemaDefinition(t, db)) != 0 {
		t.Fatal("partial initialization committed")
	}
}

func TestExistingUpgradeFailureIsAtomic(t *testing.T) {
	db := schemaTestDB(t)
	ctx := context.Background()
	scripts := []string{`CREATE TABLE retained(value TEXT); INSERT INTO retained VALUES ('synthetic');`}
	if err := migrate(ctx, db, scripts, ""); err != nil {
		t.Fatal(err)
	}
	before := schemaDefinition(t, db)
	scripts = append(scripts, `UPDATE retained SET value='lost'; CREATE TABLE partial_upgrade(value TEXT); invalid SQL`)
	if err := migrate(ctx, db, scripts, "invalid baseline must not execute"); err == nil {
		t.Fatal("broken upgrade accepted")
	}
	if !reflect.DeepEqual(before, schemaDefinition(t, db)) {
		t.Fatal("partial upgrade changed schema")
	}
	var value string
	var version int
	if err := db.QueryRow(`SELECT value FROM retained`).Scan(&value); err != nil || value != "synthetic" {
		t.Fatal("failed upgrade changed retained data", value, err)
	}
	if err := db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 1 {
		t.Fatal("failed upgrade advanced version", version, err)
	}
}
