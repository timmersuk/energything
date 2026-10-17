package storage

import (
	"context"
	"database/sql"
	"encoding/json"
)

// QueryAttemptEvidence reads selected telemetry owned by this attempt/retrieval.
// Browsing performs no startup recovery, migration or provider work.
func (s *Store) QueryAttemptEvidence(ctx context.Context, attemptID, retrievalID int64) (json.RawMessage, error) {
	if attemptID <= 0 || retrievalID <= 0 {
		return nil, ErrNotFound
	}
	var content string
	err := s.db.QueryRowContext(ctx, `SELECT e.content FROM attempts a JOIN retrievals r ON r.attempt_id=a.id AND r.identity_id=a.identity_id JOIN evidence e ON e.id=r.evidence_id WHERE a.id=? AND r.id=? AND length(CAST(e.content AS BLOB))<=4194304`, attemptID, retrievalID).Scan(&content)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Fail closed for older evidence under the existing persistence exclusion rules.
	safe, _, err := canonicalEvidence(json.RawMessage(content))
	if err != nil {
		return nil, ErrNotFound
	}
	return json.RawMessage(safe), nil
}
