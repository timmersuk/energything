package web

import (
	"context"
	"database/sql"
	"errors"

	"strings"
	"time"

	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
)

type recordStore interface {
	QueryAggregates(context.Context, storage.AggregateQuery) (storage.AggregatePage, error)
	QueryDiscovery(context.Context) ([]storage.DiscoveryState, error)
	QueryStatus(context.Context) ([]storage.Status, error)
	QueryIntervals(context.Context, storage.RecordQuery) (storage.IntervalPage, error)
	QuerySamples(context.Context, storage.SampleQuery) (storage.SamplePage, error)
}

func instant(t storage.Instant) string {
	return time.UnixMicro(int64(t)).UTC().Format(time.RFC3339Nano)
}
func nullableTime(t *storage.Instant) *string {
	if t == nil {
		return nil
	}
	s := instant(*t)
	return &s
}

type attemptView struct {
	RetryHintAt *string            `json:"provider_retry_at"`
	Diagnostic  storage.Diagnostic `json:"diagnostic"`
	ErrorCode   string             `json:"error_code,omitempty"`
	Page        *int               `json:"page,omitempty"`
	Started     string             `json:"started_at"`
	Finished    *string            `json:"finished_at"`
	Outcome     string             `json:"outcome"`
	ErrorClass  string             `json:"error_class,omitempty"`
	Message     string             `json:"message,omitempty"`
}

func attempt(a *storage.Audit) *attemptView {
	if a == nil {
		return nil
	}
	return &attemptView{RetryHintAt: nullableTime(a.Diagnostic.RetryHintAt), Diagnostic: a.Diagnostic, Started: instant(a.Started), Finished: nullableTime(a.Finished), Outcome: a.Outcome, ErrorClass: a.ErrorClass, ErrorCode: a.ErrorCode, Page: a.Page, Message: a.Message}
}

type jobView struct {
	Kind       string  `json:"kind"`
	Policy     string  `json:"policy"`
	Running    bool    `json:"running"`
	Paused     bool    `json:"paused"`
	RetryCount int     `json:"retry_count"`
	NextDue    string  `json:"next_due"`
	Checkpoint *string `json:"checkpoint"`
}
type coverageView struct {
	From                string `json:"from"`
	To                  string `json:"to"`
	Fetched             bool   `json:"fetched"`
	Expected            int    `json:"expected"`
	Valid               int    `json:"valid"`
	Missing             int    `json:"missing"`
	Qualified           int    `json:"qualified"`
	MeasurementComplete bool   `json:"measurement_complete"`
}
type sourceView struct {
	AuditCounts       *auditCounts   `json:"audit_counts,omitempty"`
	Boundary          *boundaryView  `json:"historical_boundary,omitempty"`
	PlantID           string         `json:"plant_id,omitempty"`
	MPAN              string         `json:"mpan,omitempty"`
	Serial            string         `json:"serial,omitempty"`
	IdentityID        int64          `json:"identity_id"`
	Source            string         `json:"source"`
	Stream            string         `json:"stream"`
	Identity          string         `json:"identity"`
	Selected          bool           `json:"selected"`
	RetainedCount     int64          `json:"retained_count"`
	UnresolvedCount   int64          `json:"unresolved_count"`
	LastRetrieval     *string        `json:"last_retrieved_at"`
	NewestMeasurement *string        `json:"newest_measured_at"`
	LastAttempt       *attemptView   `json:"last_attempt"`
	LastAccepted      *attemptView   `json:"last_accepted_attempt"`
	JobCount          int64          `json:"job_count"`
	WindowCount       int64          `json:"window_count"`
	Jobs              []jobView      `json:"jobs"`
	Coverage          []coverageView `json:"coverage"`
}

type boundaryView struct {
	State      string  `json:"state"`
	Oldest     *string `json:"oldest"`
	Observed   string  `json:"observed"`
	NextDue    string  `json:"next_probe_due"`
	Provenance string  `json:"provenance"`
}
type discoveryView struct {
	Failure  *storage.DiscoveryFailure `json:"failure,omitempty"`
	Stream   string                    `json:"stream"`
	State    string                    `json:"state"`
	Message  string                    `json:"message"`
	Occurred string                    `json:"occurred_at"`
}
type providerView struct {
	Streams       []discoveryView `json:"streams,omitempty"`
	Source        string          `json:"source"`
	Configuration string          `json:"configuration"`
	Message       string          `json:"message"`
	Discovery     string          `json:"discovery"`
	Sources       []sourceView    `json:"sources"`
}
type statusView struct {
	CSRFToken string         `json:"csrf_token"`
	Schedules []scheduleView `json:"schedules,omitempty"`
	Version   string         `json:"version"`
	BuildID   string         `json:"build_id"`
	Timezone  string         `json:"timezone"`
	Providers []providerView `json:"providers"`
}

type scheduleView struct {
	SearchPolicy string `json:"search_policy"`
	IdentityID   int64  `json:"identity_id"`
	Identity     string `json:"identity"`
	Kind         string `json:"kind"`
	Policy       string `json:"policy"`
	State        string `json:"state"`
	Provenance   string `json:"provenance"`
	Frontier     string `json:"frontier"`
	SearchLimit  string `json:"search_limit"`
	NextDue      string `json:"next_due"`
	CurrentJob   *int64 `json:"current_job"`
}

func statusData(ctx context.Context, store recordStore, c config.Config, build string) (statusView, error) {
	rows, err := store.QueryStatus(ctx)
	if err != nil {
		return statusView{}, err
	}
	discovery, err := store.QueryDiscovery(ctx)
	if err != nil {
		return statusView{}, err
	}
	out := statusView{Version: "v1", BuildID: build, Timezone: c.Timezone.String(), Providers: []providerView{}}
	if scheduled, ok := store.(interface {
		QuerySchedules(context.Context) ([]storage.Schedule, error)
	}); ok {
		var schedules []storage.Schedule
		schedules, err = scheduled.QuerySchedules(ctx)
		if err != nil {
			return statusView{}, err
		}
		for _, p := range schedules {
			identity := ""
			for _, i := range rows {
				if i.IdentityID == p.IdentityID {
					identity = i.Metadata.DisplayIdentity(i.Provider, i.Stream, i.IdentityID)
				}
			}
			out.Schedules = append(out.Schedules, scheduleView{SearchPolicy: p.SearchPolicy, IdentityID: p.IdentityID, Identity: identity, Kind: p.Kind, Policy: p.Policy, State: p.State, Provenance: p.Provenance, Frontier: instant(p.Frontier), SearchLimit: instant(p.SearchLimit), NextDue: instant(p.NextDue), CurrentJob: p.CurrentJob})
		}
	}
	for _, p := range []struct {
		name   string
		config config.Provider
	}{{"octopus", c.Octopus}, {"growatt", c.Growatt}} {
		v := providerView{Source: p.name, Configuration: p.config.State, Message: p.config.Message, Discovery: "No stored identities", Sources: []sourceView{}}
		if p.config.State == "valid" {
			v.Message = "Configured; automatic collection runs independently of browsing."
		}
		for _, x := range rows {
			if x.Provider != p.name {
				continue
			}
			v.Discovery = "Stored identities available; selection shown below"
			s := sourceView{IdentityID: x.IdentityID, Source: x.Provider, Stream: x.Stream, Identity: x.Metadata.DisplayIdentity(x.Provider, x.Stream, x.IdentityID), Selected: x.Selected, RetainedCount: x.IntervalCount + x.SampleCount + x.AggregateCount, UnresolvedCount: x.UnresolvedCount, LastRetrieval: nullableTime(x.LastRetrieval), NewestMeasurement: nullableTime(x.NewestMeasurement), LastAttempt: attempt(x.LastAttempt), LastAccepted: attempt(x.LastAcceptedAttempt), JobCount: x.JobCount, WindowCount: x.WindowCount, Jobs: []jobView{}, Coverage: []coverageView{}}
			if audits, ok := store.(auditStore); ok {
				page, e := audits.QueryAudit(ctx, storage.AuditQuery{IdentityID: x.IdentityID, Limit: 1})
				if e != nil {
					return statusView{}, e
				}
				s.AuditCounts = &auditCounts{page.Counts.Total, page.Counts.Failed, page.Counts.Rejected}
			}
			if boundaries, ok := store.(interface {
				HistoryBoundary(context.Context, int64) (storage.HistoryBoundary, error)
			}); ok {
				b, e := boundaries.HistoryBoundary(ctx, x.IdentityID)
				if e != nil && !errors.Is(e, sql.ErrNoRows) {
					return statusView{}, e
				}
				if e == nil {
					s.Boundary = &boundaryView{b.State, nullableTime(b.Oldest), instant(b.Observed), instant(b.NextDue), b.Provenance}
				}
			}
			if x.Provider == "octopus" {
				s.MPAN = x.Metadata.MPAN
				s.Serial = x.Metadata.Serial
				s.Identity = x.Metadata.DisplayIdentity(x.Provider, x.Stream, x.IdentityID)
			}
			if x.Provider == "growatt" {
				s.PlantID = x.Metadata.PlantID
				s.Serial = x.Metadata.Serial
			}
			for _, j := range x.Jobs {
				s.Jobs = append(s.Jobs, jobView{j.Kind, j.Policy, j.RunningAttempt != nil, j.Paused, j.RetryCount, instant(j.NextDue), nullableTime(j.Checkpoint)})
			}
			for _, w := range x.Windows {
				s.Coverage = append(s.Coverage, coverageView{instant(w.Start), instant(w.End), w.Fetched, w.Coverage.Expected, w.Coverage.Valid, w.Coverage.Missing, w.Coverage.Qualified, w.Coverage.MeasurementComplete})
			}
			v.Sources = append(v.Sources, s)
		}
		for _, d := range discovery {
			if d.Provider == p.name {
				message := providerDiscoveryMessage(d.Provider, d.Stream, d.State)
				if d.Failure != nil && (d.Failure.Class == "authentication" || d.Failure.Class == "configuration") {
					message += " Automatic discovery is paused until configuration is corrected and the server restarts."
				}
				v.Streams = append(v.Streams, discoveryView{Failure: d.Failure, Stream: d.Stream, State: d.State, Message: message, Occurred: instant(d.Occurred)})
			}
		}
		out.Providers = append(out.Providers, v)
	}
	return out, nil
}

func discoveryMessage(stream, state string) string {
	switch state {
	case "selection_required":
		return "Set paired OCTOPUS_" + strings.ToUpper(stream) + "_MPAN and OCTOPUS_" + strings.ToUpper(stream) + "_SERIAL to a matching confirmed meter, then restart discovery."
	case "conflict":
		return "Conflicting discovered meter descriptions; resolve discovery before selecting this stream."
	case "unavailable":
		return "No confirmed meter is available for this direction."
	case "failed":
		return "Discovery failed; inspect its class/code and retry state. Correct credentials or selection and restart when required."
	case "discovering":
		return "Account discovery is running."
	case "selected":
		return "A confirmed meter is selected."
	default:
		return ""
	}
}

func providerDiscoveryMessage(provider, stream, state string) string {
	if provider != "growatt" {
		return discoveryMessage(stream, state)
	}
	if stream == "generation" {
		if state == "selected" {
			return "A confirmed plant is selected for independent daily generation collection."
		}
		if state == "selection_required" {
			return "Set GROWATT_PLANT_ID to a confirmed plant, or resolve a unique device/plant selection, then restart discovery."
		}
	}
	switch state {
	case "selection_required":
		return "Inspect a private growatt-verify report and set GROWATT_PLANT_ID / GROWATT_DEVICE_SERIAL to a matching family 5 device."
	case "unavailable":
		return "No supported SPH/MIX device (family 5) was discovered."
	case "failed":
		return "Growatt discovery failed; check credentials, permission and network access. Octopus and stored browsing remain available."
	case "selected":
		return "A confirmed SPH/MIX device is selected; unresolved measurement time and field units are explicitly qualified."
	case "discovering":
		return "Read-only plant/device discovery is running."
	}
	return ""
}
