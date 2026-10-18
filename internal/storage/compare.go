package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// CompareSnapshots verifies all table values and declared identities without
// migrating either database. Only use this on offline copies/closed snapshots.
func CompareSnapshots(ctx context.Context, first, second string) error {
	db, e := sql.Open("sqlite", snapshotURI(first, "mode=ro&immutable=1"))
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.ExecContext(ctx, "ATTACH DATABASE ? AS restored", snapshotURI(second, "mode=ro&immutable=1")); e != nil {
		return e
	}
	var count int
	for _, pair := range [][2]string{{"main", "restored"}, {"restored", "main"}} {
		if e = db.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT type,name,sql FROM `+pair[0]+`.sqlite_master EXCEPT SELECT type,name,sql FROM `+pair[1]+`.sqlite_master)`).Scan(&count); e != nil || count != 0 {
			return errors.New("snapshot schemas differ")
		}
	}
	rows, e := db.QueryContext(ctx, `SELECT name FROM main.sqlite_master WHERE type='table'`)
	if e != nil {
		return e
	}
	var names []string
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			rows.Close()
			return e
		}
		names = append(names, name)
	}
	e = errors.Join(rows.Err(), rows.Close())
	if e != nil {
		return e
	}
	for _, name := range names {
		quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		var a, b int64
		if e = db.QueryRowContext(ctx, `SELECT count(*) FROM main.`+quoted).Scan(&a); e != nil {
			return e
		}
		if e = db.QueryRowContext(ctx, `SELECT count(*) FROM restored.`+quoted).Scan(&b); e != nil {
			return e
		}
		if a != b {
			return errors.New("snapshot table counts differ")
		}
		for _, pair := range [][2]string{{"main", "restored"}, {"restored", "main"}} {
			q := `SELECT count(*) FROM (SELECT * FROM ` + pair[0] + `.` + quoted + ` EXCEPT SELECT * FROM ` + pair[1] + `.` + quoted + `)`
			if e = db.QueryRowContext(ctx, q).Scan(&count); e != nil {
				return e
			}
			if count != 0 {
				return errors.New("snapshot table contents differ")
			}
		}
	}
	return nil
}
