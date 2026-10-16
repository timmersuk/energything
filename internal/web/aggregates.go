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
	if progressStore, ok := s.(interface {
		QueryGenerationProgress(context.Context, string) ([]storage.GenerationProgress, error)
	}); ok {
		rows, err := progressStore.QueryGenerationProgress(ctx, q.PlantID)
		if err != nil {
			return out, err
		}
		out.Progress = []generationProgress{}
		for _, row := range rows {
			plan, identity := row.Plan, row.Identity
			progress := generationProgress{IdentityID: identity.ID, Identity: identity.Metadata.DisplayIdentity(identity.Provider, identity.Stream, identity.ID), State: plan.State, Frontier: instant(plan.Frontier), SearchLimit: instant(plan.SearchLimit), NextDue: instant(plan.NextDue), Provenance: plan.Provenance, Jobs: []generationJob{}}
			if row.LastAttempt != nil {
				a := auditRow(*row.LastAttempt, time.UTC)
				progress.LastAttempt = &a
			}
			if j := row.CurrentJob; j != nil && (j.Checkpoint == nil || *j.Checkpoint < j.End) {
				progress.Jobs = append(progress.Jobs, generationJob{instant(j.Start), instant(j.End), instant(j.NextDue), j.Paused, j.RunningAttempt != nil, j.RetryCount})
			}
			out.Progress = append(out.Progress, progress)
		}
	}
	return out, nil
}
