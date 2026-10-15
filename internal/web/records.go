package web

import (
	"context"
	"encoding/json"

	"github.com/timmersuk/energything/internal/storage"
)

type intervalView struct {
	MPAN        string `json:"mpan"`
	Serial      string `json:"serial"`
	Source      string `json:"source"`
	IdentityID  int64  `json:"identity_id"`
	Identity    string `json:"identity"`
	Direction   string `json:"direction"`
	MeasuredAt  string `json:"measured_at"`
	End         string `json:"end"`
	RetrievedAt string `json:"retrieved_at"`
	KWh         string `json:"kwh"`
	Unit        string `json:"unit"`
	Quality     string `json:"quality"`
}
type sampleView struct {
	PlantID      string                   `json:"plant_id"`
	Serial       string                   `json:"serial"`
	ID           int64                    `json:"id"`
	Source       string                   `json:"source"`
	IdentityID   int64                    `json:"identity_id"`
	Identity     string                   `json:"identity"`
	MeasuredAt   *string                  `json:"measured_at"`
	RetrievedAt  string                   `json:"retrieved_at"`
	ProviderDate string                   `json:"provider_date"`
	OriginalTime map[string]string        `json:"original_time"`
	Quality      string                   `json:"quality"`
	Values       map[string]storage.Value `json:"values"`
}
type recordsView struct {
	Version           string      `json:"version"`
	Timezone          string      `json:"timezone"`
	View              string      `json:"view"`
	RangeBasis        string      `json:"range_basis"`
	From              *string     `json:"from"`
	To                *string     `json:"to"`
	Limit             int         `json:"limit"`
	Records           any         `json:"records"`
	NextCursor        string      `json:"next_cursor,omitempty"`
	OmittedUnresolved int64       `json:"omitted_unresolved"`
	Query             recordQuery `json:"-"`
	NextURL           string      `json:"-"`
}

func recordsData(ctx context.Context, store recordStore, q recordQuery, growatt bool, timezone, path string) (recordsView, error) {
	out := recordsView{Version: "v1", Timezone: timezone, View: "resolved", RangeBasis: "measurement", From: nullableTime(q.From), To: nullableTime(q.To), Limit: q.Limit, Query: q}
	var identities []storage.Status
	{
		var err error
		identities, err = store.QueryStatus(ctx)
		if err != nil {
			return out, err
		}
		if q.IdentityID != 0 {
			provider := "octopus"
			if growatt {
				provider = "growatt"
			}
			found := false
			for _, x := range identities {
				if x.IdentityID == q.IdentityID && x.Provider == provider {
					found = true
				}
			}
			if !found {
				return out, storage.ErrInvalid
			}
		}
	}
	if growatt {
		p, err := store.QuerySamples(ctx, q.SampleQuery)
		if err != nil {
			return out, err
		}
		devices := map[int64]storage.Status{}
		for _, identity := range identities {
			devices[identity.IdentityID] = identity
		}
		rows := []sampleView{}
		for _, x := range p.Records {
			device := devices[x.IdentityID]
			rows = append(rows, sampleView{ID: x.ID, Source: "growatt", IdentityID: x.IdentityID, Identity: device.Metadata.DisplayIdentity("growatt", device.Stream, x.IdentityID), PlantID: device.Metadata.PlantID, Serial: device.Metadata.Serial, MeasuredAt: nullableTime(x.Instant), RetrievedAt: instant(x.Retrieved), ProviderDate: x.ProviderDate, OriginalTime: x.OriginalTime, Quality: x.Quality, Values: x.Values})
		}
		out.Records = rows
		out.NextCursor = p.NextCursor
		out.OmittedUnresolved = p.OmittedUnresolved
		if q.QualityView {
			out.View = "quality"
			out.RangeBasis = "retrieval"
		}
	} else {
		p, err := store.QueryIntervals(ctx, q.RecordQuery)
		if err != nil {
			return out, err
		}
		meters := map[int64]storage.Status{}
		for _, x := range identities {
			meters[x.IdentityID] = x
		}
		rows := []intervalView{}
		for _, x := range p.Records {
			meter := meters[x.IdentityID]
			rows = append(rows, intervalView{Source: "octopus", IdentityID: x.IdentityID, Identity: meter.Metadata.DisplayIdentity("octopus", meter.Stream, x.IdentityID), Direction: meter.Stream, MPAN: meter.Metadata.MPAN, Serial: meter.Metadata.Serial, MeasuredAt: instant(x.Start), End: instant(x.End), RetrievedAt: instant(x.Retrieved), KWh: x.KWh, Unit: "kWh", Quality: "accepted"})
		}
		out.Records = rows
		out.NextCursor = p.NextCursor
	}
	if out.NextCursor != "" {
		out.NextURL = q.nextURL(path, out.NextCursor)
	}
	return out, nil
}

// ProviderClock is display text only; it does not resolve the source timezone.
func (s sampleView) ProviderClock() string {
	raw := s.OriginalTime["time"]
	if raw != "" {
		var clock string
		if json.Unmarshal([]byte(raw), &clock) == nil {
			return clock
		}
		return raw
	}
	if s.ProviderDate != "" {
		return s.ProviderDate + " (clock unavailable)"
	}
	return "Not reported"
}
