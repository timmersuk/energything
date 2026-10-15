package collection

import (
	"context"
	"database/sql"
	"errors"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

func (s *Scheduler) probeBoundaries(ctx context.Context) error {
	ids, e := s.DB.Identities(ctx)
	if e != nil {
		return e
	}
	for _, i := range ids {
		if i.Provider != "octopus" || !i.Selected {
			continue
		}
		previous, e := s.DB.HistoryBoundary(ctx, i.ID)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e == nil && previous.NextDue > storage.At(s.Now()) {
			continue
		}
		observed := s.Now()
		oldest, evidence, fetchErr := s.Octopus.Oldest(ctx, octopus.Meter{MPAN: i.Metadata.MPAN, Serial: i.Metadata.Serial, Direction: i.Stream}, observed.UTC().Truncate(30*time.Minute))
		b := storage.HistoryBoundary{IdentityID: i.ID, Observed: storage.At(observed), NextDue: storage.At(observed.Add(7 * 24 * time.Hour)), State: "observed", Provenance: "Octopus explicit year0001 domain; documented ascending period ordering; one-record probe; scoped to identity and observation time", Evidence: evidence}
		if oldest != nil {
			v := storage.At(*oldest)
			b.Oldest = &v
		}
		if fetchErr != nil {
			delay, paused := discoveryRetry(fetchErr, observed)
			b.State = "boundary_unavailable"
			b.Provenance = "Oldest-reading probe failed; recent collection continues; no absence or boundary inferred"
			b.Evidence = []byte(`{"probe":"oldest_explicit_year0001","failed":true}`)
			b.NextDue = storage.At(observed.Add(delay))
			if paused {
				b.State = "paused"
				b.NextDue = storage.At(time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC))
			}
			b.Oldest = previous.Oldest
		}
		if e = s.DB.RecordHistoryBoundary(ctx, b); e != nil {
			return e
		}
	}
	return nil
}
