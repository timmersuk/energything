package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/timmersuk/energything/internal/storage"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type auditStore interface {
	QueryAudit(context.Context, storage.AuditQuery) (storage.AuditPage, error)
	QueryAttempt(context.Context, int64) (storage.Audit, error)
	QueryAttemptEvidence(context.Context, int64, int64) (json.RawMessage, error)
}
type auditEvidence struct {
	RetrievalID int64  `json:"retrieval_id"`
	EvidenceID  int64  `json:"evidence_id"`
	Digest      string `json:"digest"`
	Retrieved   string `json:"retrieved_at"`
	Complete    bool   `json:"complete"`
	URL         string `json:"url"`
}
type auditRecord struct {
	ID               int64                     `json:"id"`
	JobID            int64                     `json:"job_id"`
	IdentityID       int64                     `json:"identity_id"`
	Source           string                    `json:"source"`
	Stream           string                    `json:"stream"`
	Identity         string                    `json:"identity"`
	ProviderIdentity storage.DiscoveryMetadata `json:"provider_identity"`
	Kind             string                    `json:"kind"`
	From             string                    `json:"requested_from"`
	To               string                    `json:"requested_to"`
	ProviderDate     string                    `json:"provider_date,omitempty"`
	Started          string                    `json:"started_at"`
	Occurred         string                    `json:"occurred_at"`
	Finished         *string                   `json:"finished_at"`
	LocalOccurred    string                    `json:"occurred_local"`
	Outcome          string                    `json:"outcome"`
	Label            string                    `json:"outcome_label"`
	ErrorClass       string                    `json:"error_class,omitempty"`
	ErrorCode        string                    `json:"error_code,omitempty"`
	Message          string                    `json:"message,omitempty"`
	Diagnostic       storage.Diagnostic        `json:"diagnostic"`
	Page             *int                      `json:"page"`
	RetryCount       int                       `json:"retry_count"`
	NextRetry        *string                   `json:"next_retry_at"`
	Retries          []storage.RetryDetail     `json:"retries"`
	AcceptedRows     int                       `json:"accepted_rows"`
	Evidence         []auditEvidence           `json:"evidence"`
	DetailURL        string                    `json:"detail_url"`
}

func auditRow(a storage.Audit, loc *time.Location) auditRecord {
	label := a.Outcome
	if a.ErrorClass == "rate_limit" {
		label = "Rate-limit deferral (" + a.Outcome + ")"
	}
	r := auditRecord{ID: a.ID, JobID: a.JobID, IdentityID: a.IdentityID, Source: a.Provider, Stream: a.Stream, Identity: a.DisplayIdentity, ProviderIdentity: a.Metadata, Kind: a.Kind, From: instant(a.Start), To: instant(a.End), ProviderDate: a.ProviderDate, Started: instant(a.Started), Occurred: instant(a.Occurred), Finished: nullableTime(a.Finished), LocalOccurred: time.UnixMicro(int64(a.Occurred)).In(loc).Format("2006-01-02 15:04:05.999999 -07:00 MST"), Outcome: a.Outcome, Label: label, ErrorClass: a.ErrorClass, ErrorCode: a.ErrorCode, Message: a.Message, Diagnostic: a.Diagnostic, Page: a.Page, RetryCount: a.RetryCount, NextRetry: nullableTime(a.NextRetry), Retries: a.Retries, AcceptedRows: a.AcceptedRows, Evidence: []auditEvidence{}, DetailURL: fmt.Sprintf("/audit/%d", a.ID)}
	if r.Retries == nil {
		r.Retries = []storage.RetryDetail{}
	}
	for _, e := range a.Evidence {
		r.Evidence = append(r.Evidence, auditEvidence{e.RetrievalID, e.EvidenceID, e.Digest, instant(e.Retrieved), e.Complete, fmt.Sprintf("/api/v1/audit/%d/evidence/%d", a.ID, e.RetrievalID)})
	}
	return r
}

type auditCounts struct {
	Total    int64 `json:"total"`
	Failed   int64 `json:"failed"`
	Rejected int64 `json:"rejected"`
}
type auditView struct {
	Version    string        `json:"version"`
	Timezone   string        `json:"timezone"`
	RangeBasis string        `json:"range_basis"`
	From       *string       `json:"from"`
	To         *string       `json:"to"`
	Limit      int           `json:"limit"`
	Records    []auditRecord `json:"records"`
	Counts     auditCounts   `json:"counts"`
	NextCursor string        `json:"next_cursor,omitempty"`
	NextURL    string        `json:"-"`
	Filters    url.Values    `json:"-"`
	Detail     bool          `json:"-"`
}

func parseAudit(v url.Values, loc *time.Location) (storage.AuditQuery, error) {
	q := storage.AuditQuery{Provider: v.Get("source"), Stream: v.Get("stream"), Outcome: v.Get("outcome"), ErrorClass: v.Get("error_class")}
	base := url.Values{}
	for k, values := range v {
		if len(values) != 1 {
			return q, storage.ErrInvalid
		}
		switch k {
		case "source", "stream", "outcome", "error_class":
		case "identity", "from", "to", "limit", "cursor":
			base[k] = values
		default:
			return q, storage.ErrInvalid
		}
	}
	r, err := parseQuery(base, false, loc, time.Now())
	if err != nil {
		return q, storage.ErrInvalid
	}
	q.IdentityID = r.IdentityID
	q.Limit = r.Limit
	q.Cursor = r.Cursor
	// Default to all retained audit; explicit ranges use record calendar semantics.
	if v.Get("from") != "" || v.Get("to") != "" {
		q.From = r.From
		q.To = r.To
	}
	return q, nil
}
func auditData(ctx context.Context, s recordStore, v url.Values, path string, loc *time.Location) (auditView, error) {
	out := auditView{Version: "v1", Timezone: loc.String(), RangeBasis: "attempt_occurrence", Records: []auditRecord{}, Filters: v}
	db, ok := s.(auditStore)
	if !ok {
		return out, fmt.Errorf("audit storage unavailable")
	}
	if path != "/audit" && path != "/api/v1/audit" {
		if len(v) != 0 {
			return out, storage.ErrInvalid
		}
		raw := strings.TrimPrefix(strings.TrimPrefix(path, "/api/v1"), "/audit/")
		id, err := auditID(raw)
		if err != nil {
			return out, err
		}
		a, err := db.QueryAttempt(ctx, id)
		if err != nil {
			return out, err
		}
		out.Detail = true
		out.Records = append(out.Records, auditRow(a, loc))
		return out, nil
	}
	q, err := parseAudit(v, loc)
	if err != nil {
		return out, err
	}
	if q.IdentityID != 0 {
		identities, err := s.QueryStatus(ctx)
		if err != nil {
			return out, err
		}
		found := false
		for _, i := range identities {
			if i.IdentityID == q.IdentityID && (q.Provider == "" || i.Provider == q.Provider) && (q.Stream == "" || i.Stream == q.Stream) {
				found = true
			}
		}
		if !found {
			return out, storage.ErrInvalid
		}
	}
	p, err := db.QueryAudit(ctx, q)
	if err != nil {
		return out, err
	}
	out.From = nullableTime(q.From)
	out.To = nullableTime(q.To)
	out.Limit = q.Limit
	out.Counts = auditCounts{p.Counts.Total, p.Counts.Failed, p.Counts.Rejected}
	for _, a := range p.Records {
		out.Records = append(out.Records, auditRow(a, loc))
	}
	out.NextCursor = p.NextCursor
	if p.NextCursor != "" {
		next := url.Values{}
		for k, x := range v {
			next[k] = append([]string(nil), x...)
		}
		next.Set("cursor", p.NextCursor)
		out.NextURL = path + "?" + next.Encode()
	}
	return out, nil
}
func auditID(raw string) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != raw {
		return 0, storage.ErrNotFound
	}
	return n, nil
}
func auditEvidenceResponse(w http.ResponseWriter, r *http.Request, s recordStore, v url.Values) error {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/audit/"), "/")
	if len(parts) != 3 || parts[1] != "evidence" {
		return storage.ErrNotFound
	}
	for k, values := range v {
		if k != "download" || len(values) != 1 || values[0] != "1" {
			return storage.ErrInvalid
		}
	}
	a, err := auditID(parts[0])
	if err != nil {
		return err
	}
	id, err := auditID(parts[2])
	if err != nil {
		return err
	}
	db, ok := s.(auditStore)
	if !ok {
		return fmt.Errorf("audit storage unavailable")
	}
	content, err := db.QueryAttemptEvidence(r.Context(), a, id)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if v.Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="attempt-%d-retrieval-%d.json"`, a, id))
	}
	return json.NewEncoder(w).Encode(content)
}
