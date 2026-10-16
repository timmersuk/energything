package web

import (
	"context"
	"github.com/timmersuk/energything/internal/storage"
	"net/url"
	"strconv"
	"time"
)

type aggregateView struct {
	Source      string  `json:"source"`
	PeriodStart *string `json:"period_start"`
	PeriodEnd   *string `json:"period_end"`
	storage.Aggregate
	RetrievedAt string `json:"retrieved_at"`
}
type aggregatesView struct {
	Progress          []generationProgress `json:"collection_progress"`
	Version           string               `json:"version"`
	Records           []aggregateView      `json:"records"`
	NextCursor        string               `json:"next_cursor,omitempty"`
	NextURL           string               `json:"-"`
	From, To, PlantID string               `json:"-"`
}

type generationProgress struct {
	IdentityID  int64           `json:"identity_id"`
	Identity    string          `json:"identity"`
	State       string          `json:"state"`
	Frontier    string          `json:"frontier"`
	SearchLimit string          `json:"search_limit"`
	NextDue     string          `json:"next_due"`
	Provenance  string          `json:"provenance"`
	Jobs        []generationJob `json:"unfinished_jobs"`
	LastAttempt *auditRecord    `json:"last_attempt"`
}
type generationJob struct {
	From    string `json:"requested_from"`
	To      string `json:"requested_to"`
	NextDue string `json:"next_due"`
	Paused  bool   `json:"paused"`
	Running bool   `json:"running"`
	Retries int    `json:"retry_count"`
}

func aggregateData(ctx context.Context, s recordStore, v url.Values, path string) (aggregatesView, error) {
	out := aggregatesView{Version: "v1", Records: []aggregateView{}}
	for k, values := range v {
		if len(values) != 1 {
			return out, storage.ErrInvalid
		}
		switch k {
		case "plant_id", "metric", "resolution", "from", "to", "limit", "cursor":
		default:
			return out, storage.ErrInvalid
		}
	}
	q := storage.AggregateQuery{PlantID: v.Get("plant_id"), Metric: v.Get("metric"), Resolution: v.Get("resolution"), From: v.Get("from"), To: v.Get("to"), Cursor: v.Get("cursor")}
	if v.Has("limit") {
		n, e := strconv.Atoi(v.Get("limit"))
		if e != nil || n < 1 || n > 500 {
			return out, storage.ErrInvalid
		}
		q.Limit = n
	}
	p, e := s.QueryAggregates(ctx, q)
	if e != nil {
		return out, e
	}
	for _, x := range p.Records {
		out.Records = append(out.Records, aggregateView{Source: "growatt", Aggregate: x, RetrievedAt: instant(x.Retrieved)})
	}
	out.From = q.From
	out.To = q.To
	out.PlantID = q.PlantID
	out.NextCursor = p.NextCursor
	if p.NextCursor != "" {
		v.Set("cursor", p.NextCursor)
		out.NextURL = path + "?" + v.Encode()
	}
	if schedules, ok := s.(interface {
		QuerySchedules(context.Context) ([]storage.Schedule, error)
	}); ok {
		plans, err := schedules.QuerySchedules(ctx)
		if err != nil {
			return out, err
		}
		states, err := s.QueryStatus(ctx)
		if err != nil {
			return out, err
		}
		out.Progress = []generationProgress{}
		for _, plan := range plans {
			if plan.Kind != "aggregate" || plan.Policy != "backfill" {
				continue
			}
			for _, state := range states {
				if state.IdentityID != plan.IdentityID || (q.PlantID != "" && state.Metadata.PlantID != q.PlantID) {
					continue
				}
				progress := generationProgress{IdentityID: state.IdentityID, Identity: state.Metadata.DisplayIdentity(state.Provider, state.Stream, state.IdentityID), State: plan.State, Frontier: instant(plan.Frontier), SearchLimit: instant(plan.SearchLimit), NextDue: instant(plan.NextDue), Provenance: plan.Provenance, Jobs: []generationJob{}}
				if state.LastAttempt != nil {
					a := auditRow(*state.LastAttempt, time.UTC)
					progress.LastAttempt = &a
				}
				for _, job := range state.Jobs {
					if job.Checkpoint != nil && *job.Checkpoint >= job.End {
						continue
					}
					progress.Jobs = append(progress.Jobs, generationJob{instant(job.Start), instant(job.End), instant(job.NextDue), job.Paused, job.RunningAttempt != nil, job.RetryCount})
				}
				out.Progress = append(out.Progress, progress)
			}
		}
	}
	return out, nil
}
