package storage

import (
	"context"
	"testing"
	"time"
)

func TestDiscoveryAtomicSelectionAndVersionOneUpgrade(t *testing.T) {
	ctx := context.Background()
	db, dir, identity, _ := fixture(t, "octopus")
	// Simulate the real task 03 schema: existing data, only migration 001 installed.
	if _, err := db.db.Exec(`DROP TABLE discovery_states;DROP TABLE growatt_discovery;ALTER TABLE attempts DROP COLUMN diagnostic;DELETE FROM schema_migrations WHERE version>=2`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	status, err := db.QueryStatus(ctx)
	if err != nil || len(status) != 1 || status[0].IdentityID != identity.ID {
		t.Fatal("upgrade lost identity", status, err)
	}
	states := []DiscoveryState{{"octopus", "import", "selection_required", At(time.Now()), nil}, {"octopus", "export", "selected", At(time.Now()), nil}}
	identities, err := db.RecordOctopusDiscovery(ctx, states, []Identity{{Provider: "octopus", Stream: "export", StableKey: "synthetic-export", Selected: true}})
	if err != nil || len(identities) != 1 {
		t.Fatal(err)
	}
	status, _ = db.QueryStatus(ctx)
	if status[0].Selected || !status[1].Selected {
		t.Fatal("stale selection", status)
	}
	if _, err = db.RecordOctopusDiscovery(ctx, states, []Identity{{Provider: "octopus", Stream: "invalid", StableKey: "synthetic", Selected: true}}); err == nil {
		t.Fatal("invalid identity accepted")
	}
	status, _ = db.QueryStatus(ctx)
	if !status[1].Selected {
		t.Fatal("selection transaction did not rollback")
	}
	got, err := db.QueryDiscovery(ctx)
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
}
