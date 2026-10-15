package web

import (
	"context"
	"github.com/timmersuk/energything/internal/storage"
	"net/url"
	"strconv"
)

type aggregateView struct {
	Source      string  `json:"source"`
	PeriodStart *string `json:"period_start"`
	PeriodEnd   *string `json:"period_end"`
	storage.Aggregate
	RetrievedAt string `json:"retrieved_at"`
}
type aggregatesView struct {
	Version           string          `json:"version"`
	Records           []aggregateView `json:"records"`
	NextCursor        string          `json:"next_cursor,omitempty"`
	NextURL           string          `json:"-"`
	From, To, PlantID string          `json:"-"`
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
	return out, nil
}
