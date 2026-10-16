package storage

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalReportedPeriodReplayCorrectionReopen(t *testing.T) {
	ctx := context.Background()
	s, dir, _, job := fixture(t, "growatt")
	for _, value := range []string{"1", "1", "2"} {
		in := AcceptedWindow{AttemptID: begin(t, s, job), Finished: testStart + 4*halfHour,
			Evidence: []Evidence{{Content: json.RawMessage(`{"synthetic":true}`), Retrieved: testStart + 4*halfHour, Complete: true}},
			Samples:  []Sample{{ProviderDate: job.ProviderDate, OriginalTime: map[string]string{"time": `"2026-10-01 12:00:00"`}, Quality: "unresolved", Values: map[string]Value{"soc": {Decimal: ptr(value), Quality: "valid"}}}}}
		if err := s.CommitAcceptedWindow(ctx, in); err != nil {
			t.Fatal(err)
		}
		if count(t, s, "samples") != 1 {
			t.Fatal("refetch duplicated a reported period")
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.QuerySamples(ctx, SampleQuery{QualityView: true})
	if err != nil || len(p.Records) != 1 || *p.Records[0].Values["soc"].Decimal != "2" || p.Records[0].Instant != nil {
		t.Fatal("canonical correction not retained", p, err)
	}
	var n int
	if err = s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('sample_versions','interval_versions','aggregate_versions')`).Scan(&n); err != nil || n != 0 {
		t.Fatal("value history tables remain", n, err)
	}
	// The source period, not a fetch or a synthesized reading key, is the PK.
	rows, err := s.db.Query(`PRAGMA table_info(samples)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if err = rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			t.Fatal(err)
		}
		if pk > 0 {
			keys = append(keys, name)
		}
	}
	if strings.Join(keys, ",") != "identity_id,provider_date,provider_time" {
		t.Fatal("wrong canonical primary key", keys)
	}
}

// Historical migration tests must actually reconstruct the old samples schema,
// rather than merely deleting migration bookkeeping from the new schema.
func restoreSchemaSeven(t *testing.T, s *Store) {
	t.Helper()
	one, _ := migrations.ReadFile("migrations/001_initial.sql")
	five, _ := migrations.ReadFile("migrations/005_aggregates.sql")
	text := string(one)
	agg := string(five)
	sql := `DROP TABLE samples;` + text[strings.Index(text, "CREATE TABLE interval_versions"):strings.Index(text, "CREATE TABLE completions")] + agg[strings.Index(agg, "CREATE TABLE aggregate_versions"):strings.Index(agg, "CREATE TABLE aggregate_calls")] + `DELETE FROM schema_migrations WHERE version=8;`
	if _, err := s.db.Exec(sql); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalMigrationCollapsesLegacyReplication(t *testing.T) {
	ctx := context.Background()
	s, dir, _, job := fixture(t, "growatt")
	restoreSchemaSeven(t, s)
	aid := begin(t, s, job)
	if err := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: aid, Finished: testStart + 4*halfHour, Evidence: []Evidence{{Content: json.RawMessage(`{"synthetic":true}`), Retrieved: testStart + 4*halfHour, Complete: true}}}); err != nil {
		t.Fatal(err)
	}
	for i, value := range []string{"1", "1", "2"} {
		_, err := s.db.Exec(`INSERT INTO samples(identity_id,attempt_id,ordinal,provider_date,original_time,quality,values_json,retrieved,retrieval_id) VALUES(?,?,?,?,?,?,?,?,(SELECT max(id) FROM retrievals))`, job.IdentityID, aid, i, job.ProviderDate, `{"time":"\"2026-10-01 12:00:00\""}`, "unresolved", `{"soc":{"value":"`+value+`","unit":null,"quality":"valid"}}`, testStart+Instant(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.QuerySamples(ctx, SampleQuery{QualityView: true})
	if err != nil || len(p.Records) != 1 || *p.Records[0].Values["soc"].Decimal != "2" {
		t.Fatal("upgrade kept replication or wrong canonical value", p, err)
	}
	if count(t, s, "attempts") != 1 || count(t, s, "retrievals") != 1 {
		t.Fatal("upgrade lost audit/evidence")
	}
}
