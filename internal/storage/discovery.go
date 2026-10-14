package storage

import (
	"context"
	"database/sql"
	"encoding/json"
)

type DiscoveryState struct {
	Provider, Stream, State string
	Occurred                Instant
}

// RecordOctopusDiscovery updates both streams and selection atomically. Old
// identities remain queryable, but a stale identity cannot remain selected.
func (s *Store) RecordOctopusDiscovery(ctx context.Context, states []DiscoveryState, identities []Identity) ([]Identity, error) {
	if len(states) != 2 {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	for _, x := range states {
		if x.Provider != "octopus" || (x.Stream != "import" && x.Stream != "export") || seen[x.Stream] || !validInstant(x.Occurred) {
			return nil, ErrInvalid
		}
		seen[x.Stream] = true
		switch x.State {
		case "discovering", "selected", "unavailable", "selection_required", "conflict", "failed":
		default:
			return nil, ErrInvalid
		}
	}
	out := append([]Identity(nil), identities...)
	err := s.write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE identities SET selected=0 WHERE provider='octopus'`); err != nil {
			return err
		}
		for i, x := range out {
			if x.Provider != "octopus" || !validText(x.StableKey) || (x.Stream != "import" && x.Stream != "export") {
				return ErrInvalid
			}
			metadata, err := marshalDiscovery(x.Metadata)
			if err != nil {
				return err
			}
			if err = tx.QueryRowContext(ctx, `INSERT INTO identities(provider,stream,stable_key,selected,metadata) VALUES(?,?,?,?,?) ON CONFLICT(provider,stream,stable_key) DO UPDATE SET selected=excluded.selected,metadata=excluded.metadata RETURNING id`, x.Provider, x.Stream, x.StableKey, x.Selected, metadata).Scan(&out[i].ID); err != nil {
				return err
			}
		}
		for _, x := range states {
			if _, err := tx.ExecContext(ctx, `INSERT INTO discovery_states VALUES(?,?,?,?) ON CONFLICT(provider,stream) DO UPDATE SET state=excluded.state,occurred=excluded.occurred`, x.Provider, x.Stream, x.State, x.Occurred); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}
func (s *Store) QueryDiscovery(ctx context.Context) ([]DiscoveryState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider,stream,state,occurred FROM discovery_states ORDER BY provider,stream`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DiscoveryState{}
	for rows.Next() {
		var x DiscoveryState
		if err := rows.Scan(&x.Provider, &x.Stream, &x.State, &x.Occurred); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func marshalDiscovery(m DiscoveryMetadata) (string, error) {
	b, e := json.Marshal(m)
	return string(b), e
}
