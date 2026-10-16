package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Telemetry must be selected/sanitized upstream; fail closed on transcript fields.
func canonicalEvidence(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 || len(raw) > 4*1024*1024 {
		return "", "", ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if !json.Valid(raw) || dec.Decode(&v) != nil {
		return "", "", ErrInvalid
	}
	var check func(any) bool
	check = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				key := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(k))
				for _, bad := range []string{"token", "password", "secret", "authorization", "apikey", "cookie", "header", "address", "account", "credential", "url", "exception", "stacktrace"} {
					if strings.Contains(key, bad) {
						return false
					}
				}
				if !check(v) {
					return false
				}
			}
		case []any:
			for _, v := range x {
				if !check(v) {
					return false
				}
			}
		}
		return true
	}
	if !check(v) {
		return "", "", fmt.Errorf("evidence contains forbidden transcript/discovery fields: %w", ErrInvalid)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", "", ErrInvalid
	}
	d := sha256.Sum256(b)
	return string(b), hex.EncodeToString(d[:]), nil
}

type preparedEvidence struct {
	Evidence
	content, digest string
}

func prepareEvidence(in []Evidence) ([]preparedEvidence, error) {
	if len(in) > 100 {
		return nil, ErrInvalid
	}
	out := make([]preparedEvidence, len(in))
	for i, x := range in {
		content, digest, err := canonicalEvidence(x.Content)
		if err != nil {
			return nil, err
		}
		out[i] = preparedEvidence{x, content, digest}
	}
	return out, nil
}
func storeEvidence(ctx context.Context, tx *sql.Tx, a Audit, in []preparedEvidence) ([]int64, error) {
	ids := make([]int64, len(in))
	for i, x := range in {
		if x.Retrieved < a.Started || (a.Finished != nil && x.Retrieved > *a.Finished) {
			return nil, ErrInvalid
		}
		content, digest := x.content, x.digest
		var err error
		var blob int64
		if err = tx.QueryRowContext(ctx, `INSERT INTO evidence(digest,content) VALUES(?,?) ON CONFLICT(digest) DO UPDATE SET digest=excluded.digest RETURNING id`, digest, content).Scan(&blob); err != nil {
			return nil, err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO retrievals(evidence_id,identity_id,attempt_id,retrieved,complete) VALUES(?,?,?,?,?)
   ON CONFLICT(attempt_id,evidence_id,retrieved,complete) DO UPDATE SET complete=excluded.complete RETURNING id`, blob, a.IdentityID, a.ID, x.Retrieved, x.Complete).Scan(&ids[i])
		if err != nil {
			return nil, err
		}
	}
	return ids, nil
}
func runningAttempt(ctx context.Context, tx *sql.Tx, id int64, finished Instant) (Audit, error) {
	var a Audit
	err := tx.QueryRowContext(ctx, `SELECT id,job_id,identity_id,kind,start,end,provider_date,started,outcome FROM attempts WHERE id=?`, id).Scan(&a.ID, &a.JobID, &a.IdentityID, &a.Kind, &a.Start, &a.End, &a.ProviderDate, &a.Started, &a.Outcome)
	if err != nil {
		return a, err
	}
	if a.Outcome != "running" || !validInstant(finished) || finished < a.Started {
		return a, ErrConflict
	}
	a.Finished = &finished
	return a, nil
}
func sameDecimal(a, b string) bool {
	x, ok := new(big.Rat).SetString(a)
	if !ok {
		return false
	}
	y, ok := new(big.Rat).SetString(b)
	return ok && x.Cmp(y) == 0
}
func validateAccepted(a Audit, provider string, in AcceptedWindow) ([]Interval, error) {
	if len(in.Intervals)+len(in.Samples)+len(in.Aggregates) > 10000 || len(in.Evidence) == 0 || len(in.Evidence) > 100 || validateRetries(in.Retries) != nil {
		return nil, ErrInvalid
	}
	for _, e := range in.Evidence {
		if !e.Complete {
			return nil, ErrInvalid
		}
	}
	if provider == "octopus" && len(in.Samples) > 0 || provider == "growatt" && len(in.Intervals) > 0 {
		return nil, ErrInvalid
	}
	if a.Kind == "aggregate" {
		if provider != "growatt" || len(in.Intervals)+len(in.Samples) != 0 {
			return nil, ErrInvalid
		}
		if _, err := validateAggregates(a, in); err != nil {
			return nil, err
		}
	} else if len(in.Aggregates) != 0 {
		return nil, ErrInvalid
	}
	intervals := append([]Interval(nil), in.Intervals...)
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start < intervals[j].Start })
	out := []Interval{}
	const slot = Instant(30 * 60 * 1e6)
	for _, x := range intervals {
		if (x.IdentityID != 0 && x.IdentityID != a.IdentityID) || len(x.KWh) > 128 || !decimal.MatchString(x.KWh) || x.Start < a.Start || x.End > a.End || x.Start%slot != 0 || x.End-x.Start != slot || x.End > in.Finished || x.Evidence < 0 || x.Evidence >= len(in.Evidence) {
			return nil, ErrInvalid
		}
		if len(out) > 0 {
			p := out[len(out)-1]
			if x.Start == p.Start && x.End == p.End && sameDecimal(x.KWh, p.KWh) {
				continue
			}
			if x.Start < p.End {
				return nil, fmt.Errorf("conflicting or overlapping interval: %w", ErrInvalid)
			}
		}
		out = append(out, x)
	}
	keys := map[string]bool{}
	for _, x := range in.Samples {
		if (x.IdentityID != 0 && x.IdentityID != a.IdentityID) || !validDate(x.ProviderDate) || !validText(x.Quality) || x.Evidence < 0 || x.Evidence >= len(in.Evidence) || x.OriginalTime == nil || x.Values == nil {
			return nil, ErrInvalid
		}
		if x.ProviderDate != a.ProviderDate && a.ProviderDate != "" {
			return nil, ErrInvalid
		}
		if x.ResolvedKey != nil {
			if !validText(*x.ResolvedKey) || keys[*x.ResolvedKey] {
				return nil, ErrInvalid
			}
			keys[*x.ResolvedKey] = true
		}
		if x.Instant != nil && (*x.Instant < a.Start || *x.Instant >= a.End) {
			return nil, ErrInvalid
		}
		for k, v := range x.Values {
			if !validText(k) || (v.Unit != nil && !validText(*v.Unit)) || !validText(v.Quality) || (v.Decimal != nil && (len(*v.Decimal) > 128 || !signedDecimal.MatchString(*v.Decimal))) {
				return nil, ErrInvalid
			}
		}
		original, _ := json.Marshal(x.OriginalTime)
		values, _ := json.Marshal(x.Values)
		if _, _, err := canonicalEvidence(original); err != nil {
			return nil, err
		}
		if _, _, err := canonicalEvidence(values); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) CommitAcceptedWindow(ctx context.Context, in AcceptedWindow) error {
	prepared, err := prepareEvidence(in.Evidence)
	if err != nil {
		return err
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		a, err := runningAttempt(ctx, tx, in.AttemptID, in.Finished)
		if err != nil {
			return err
		}
		var provider, stream, metadata string
		if err = tx.QueryRowContext(ctx, `SELECT provider,stream,metadata FROM identities WHERE id=?`, a.IdentityID).Scan(&provider, &stream, &metadata); err != nil {
			return err
		}
		if a.Kind == "aggregate" {
			var m DiscoveryMetadata
			if json.Unmarshal([]byte(metadata), &m) != nil || stream != "generation" || m.PlantID == "" || m.Serial != "" {
				return ErrInvalid
			}
			for _, x := range in.Aggregates {
				if x.PlantID != m.PlantID {
					return ErrInvalid
				}
			}
		}
		intervals, err := validateAccepted(a, provider, in)
		if err != nil {
			return err
		}
		ids, err := storeEvidence(ctx, tx, a, prepared)
		if err != nil {
			return err
		}
		if s.fault != nil {
			if err = s.fault("after_evidence"); err != nil {
				return err
			}
		}
		for _, x := range intervals {
			retrieval := ids[x.Evidence]
			retrieved := in.Evidence[x.Evidence].Retrieved
			_, err = tx.ExecContext(ctx, `INSERT INTO intervals(identity_id,start,end,kwh,retrieved,retrieval_id) VALUES(?,?,?,?,?,?)
    ON CONFLICT(identity_id,start) DO UPDATE SET end=excluded.end,kwh=excluded.kwh,retrieved=excluded.retrieved,retrieval_id=excluded.retrieval_id`, a.IdentityID, x.Start, x.End, x.KWh, retrieved, retrieval)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO interval_versions(identity_id,start,end,kwh,retrieval_id) VALUES(?,?,?,?,?)`, a.IdentityID, x.Start, x.End, x.KWh, retrieval); err != nil {
				return err
			}
		}
		aggregates, err := validateAggregates(a, in)
		if err != nil {
			return err
		}
		for _, x := range aggregates {
			retrieval := ids[x.Evidence]
			args := []any{a.IdentityID, x.Metric, x.Resolution, x.Period, x.Value, x.Unit, x.Calendar, x.Quality, in.Evidence[x.Evidence].Retrieved, retrieval}
			if _, err = tx.ExecContext(ctx, `INSERT INTO energy_aggregates VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(identity_id,metric,resolution,period) DO UPDATE SET value=excluded.value,unit=excluded.unit,calendar=excluded.calendar,quality=excluded.quality,retrieved=excluded.retrieved,retrieval_id=excluded.retrieval_id`, args...); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO aggregate_versions(identity_id,metric,resolution,period,value,unit,calendar,quality,retrieval_id) VALUES(?,?,?,?,?,?,?,?,?)`, append(args[:8], retrieval)...); err != nil {
				return err
			}
		}
		for ordinal, x := range in.Samples {
			original, _ := json.Marshal(x.OriginalTime)
			values, _ := json.Marshal(x.Values)
			retrieval := ids[x.Evidence]
			var id int64
			err = tx.QueryRowContext(ctx, `INSERT INTO samples(identity_id,resolved_key,attempt_id,ordinal,instant,provider_date,original_time,quality,values_json,retrieved,retrieval_id)
    VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(identity_id,resolved_key) DO UPDATE SET
    attempt_id=excluded.attempt_id,ordinal=excluded.ordinal,instant=excluded.instant,provider_date=excluded.provider_date,
    original_time=excluded.original_time,quality=excluded.quality,values_json=excluded.values_json,retrieved=excluded.retrieved,retrieval_id=excluded.retrieval_id RETURNING id`,
				a.IdentityID, x.ResolvedKey, a.ID, ordinal, x.Instant, x.ProviderDate, string(original), x.Quality, string(values), in.Evidence[x.Evidence].Retrieved, retrieval).Scan(&id)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO sample_versions(sample_id,retrieval_id,resolved_key,instant,original_time,quality,values_json) VALUES(?,?,?,?,?,?,?)`, id, retrieval, x.ResolvedKey, x.Instant, string(original), x.Quality, string(values))
			if err != nil {
				return err
			}
		}
		if s.fault != nil {
			if err = s.fault("after_observations"); err != nil {
				return err
			}
		}
		coverage := in.Coverage
		if a.Kind == "aggregate" {
			coverage = Coverage{Valid: len(aggregates), Qualified: len(aggregates)}
		}
		if provider == "octopus" {
			elapsed := a.End
			if in.Finished < elapsed {
				elapsed = in.Finished
			}
			if elapsed < a.Start {
				elapsed = a.Start
			}
			coverage = Coverage{Expected: int((elapsed - a.Start) / Instant(30*60*1e6)), Valid: len(intervals)}
			coverage.Missing = coverage.Expected - coverage.Valid
			coverage.MeasurementComplete = coverage.Expected > 0 && coverage.Missing == 0
		}
		rows := len(intervals) + len(in.Samples) + len(aggregates)
		if coverage.Expected < 0 || coverage.Valid < 0 || coverage.Missing < 0 || coverage.Qualified < 0 || coverage.Valid > rows || coverage.Qualified > coverage.Valid || ((provider == "octopus" || coverage.Expected > 0) && coverage.Expected != coverage.Valid+coverage.Missing) || (coverage.MeasurementComplete && (coverage.Expected == 0 || coverage.Missing != 0 || coverage.Qualified != 0)) {
			return ErrInvalid
		}
		if rows == 0 {
			coverage = Coverage{Expected: coverage.Expected, Missing: coverage.Expected}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO completions(job_id,start,end,attempt_id,fetched,expected,valid,missing,qualified,measurement_complete) VALUES(?,?,?,?,1,?,?,?,?,?)
   ON CONFLICT(job_id,start,end) DO UPDATE SET attempt_id=excluded.attempt_id,fetched=1,expected=excluded.expected,valid=excluded.valid,missing=excluded.missing,qualified=excluded.qualified,measurement_complete=excluded.measurement_complete`,
			a.JobID, a.Start, a.End, a.ID, coverage.Expected, coverage.Valid, coverage.Missing, coverage.Qualified, coverage.MeasurementComplete)
		if err != nil {
			return err
		}
		outcome := "success"
		if rows == 0 {
			outcome = "empty"
		}
		retries, _ := json.Marshal(in.Retries)
		class, code, message := "", "", ""
		if in.TimestampQuality {
			class = "timestamp_quality"
			code = "unavailable"
			message = errorMessages[class]
		}
		if _, err = tx.ExecContext(ctx, `UPDATE attempts SET outcome=?,finished=?,occurred=?,accepted_rows=?,retry_details=?,error_class=?,error_code=?,message=? WHERE id=?`, outcome, in.Finished, in.Finished, rows, string(retries), class, code, message, a.ID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE jobs SET checkpoint=?,next_due=?,retry_count=0,paused=0,running_attempt=NULL WHERE id=?`, a.End, in.NextDue, a.JobID)
		return err
	})
}

var errorMessages = map[string]string{
	"configuration": "Source configuration requires attention", "selection": "Source selection requires attention",
	"authentication": "Provider authentication failed", "rate_limit": "Provider rate limit deferred collection",
	"network": "Provider transport failed", "timeout": "Provider request timed out",
	"pagination": "Provider pagination was incomplete", "protocol": "Provider response was incomplete or invalid",
	"validation": "Collection window rejected by validation", "timestamp_quality": "Provider timestamp remains unresolved",
	"storage": "Storage failed; no completion recorded",
}
var errorCodes = map[string]bool{"unsupported_family": true, "body_budget": true, "invalid_json": true, "missing_error_code": true, "provider_error": true, "http_status": true, "invalid_identity": true, "repeated_page": true, "identity_mismatch": true, "sample_date": true, "sample_decimal": true, "soc_range": true, "pv_negative": true, "pv_unit_conflict": true, "missing_count": true, "invalid_count": true, "count_budget": true, "count_changed": true, "invalid_decimal": true, "invalid_timestamp": true, "invalid_duration": true, "unaligned_interval": true, "outside_window": true, "boundary_overlap": true, "future_interval": true, "conflicting_interval": true, "count_mismatch": true, "missing_results": true, "invalid": true, "incomplete": true, "conflict": true, "unavailable": true, "unauthorized": true, "limited": true, "timeout": true, "cancelled": true, "io": true, "interrupted": true}

func validateRetries(in []RetryDetail) error {
	if len(in) > 300 {
		return ErrInvalid
	}
	for _, r := range in {
		if r.Page < 0 || r.Page > 100 || r.Retry < 0 || r.Retry > 3 || r.HTTPStatus < 0 || r.HTTPStatus > 599 || r.WaitMicroseconds < 0 || !errorCodes[r.Code] {
			return ErrInvalid
		}
	}
	return nil
}

func (s *Store) RecordRejectedOrFailedAttempt(ctx context.Context, in FailedAttempt) error {
	if in.Outcome != "failed" && in.Outcome != "rejected" && in.Outcome != "cancelled" {
		return ErrInvalid
	}
	if validateDiagnostic(in.Diagnostic) != nil {
		return ErrInvalid
	}
	diagnostic, _ := json.Marshal(in.Diagnostic)
	message, ok := errorMessages[in.ErrorClass]
	if detail, exists := diagnosticMessages[in.ErrorClass+"/"+in.ErrorCode]; exists {
		message = detail
	}
	if !ok || !errorCodes[in.ErrorCode] || len(in.Evidence) > 100 || validateRetries(in.Retries) != nil || (in.Page != nil && (*in.Page < 0 || *in.Page > 100)) {
		return ErrInvalid
	}
	prepared, err := prepareEvidence(in.Evidence)
	if err != nil {
		return err
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		a, err := runningAttempt(ctx, tx, in.AttemptID, in.Finished)
		if err != nil {
			return err
		}
		if _, err = storeEvidence(ctx, tx, a, prepared); err != nil {
			return err
		}
		retries, _ := json.Marshal(in.Retries)
		if _, err = tx.ExecContext(ctx, `UPDATE attempts SET outcome=?,finished=?,occurred=?,error_class=?,error_code=?,message=?,page=?,next_retry=?,retry_details=?,diagnostic=? WHERE id=?`,
			in.Outcome, in.Finished, in.Finished, in.ErrorClass, in.ErrorCode, message, in.Page, in.NextDue, string(retries), string(diagnostic), a.ID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE jobs SET running_attempt=NULL,next_due=?,retry_count=retry_count+1,paused=? WHERE id=?`, in.NextDue, in.Paused, a.JobID)
		return err
	})
}

var diagnosticMessages = map[string]string{
	"configuration/unsupported_family": "Only discovery family 5 (SPH/MIX) is supported",
	"protocol/body_budget":             "Provider response exceeds the 4 MiB page limit",
	"protocol/invalid_json":            "Provider response is malformed or truncated JSON",
	"protocol/missing_error_code":      "Provider response is missing an integer success/error code",
	"protocol/provider_error":          "Provider returned a nonzero error code; no telemetry accepted",
	"protocol/http_status":             "Provider returned an unexpected HTTP status; redirects are disabled",
	"protocol/invalid_identity":        "Discovery identity is missing or malformed",
	"pagination/repeated_page":         "Provider repeated a nonempty page before collection completed",
	"pagination/missing_count":         "Provider page is missing a nonnegative integer count",
	"pagination/count_budget":          "Provider count or pagination exceeds the bounded unit budget",
	"pagination/count_changed":         "Provider count changed between pages; the unit is inconsistent",
	"pagination/count_mismatch":        "Pagination stopped early or assembled rows exceed the reported count",
	"validation/identity_mismatch":     "Telemetry serial does not match the selected device",
	"validation/sample_date":           "Telemetry readable date is outside the requested provider day",
	"validation/sample_decimal":        "Known telemetry field is not a supported exact decimal",
	"validation/soc_range":             "Battery remaining capacity is outside 0 through 100",
	"validation/pv_negative":           "Verified PV input power is negative",
	"validation/pv_unit_conflict":      "PV input power conflicts with the provider W display label",
	"protocol/missing_count":           "Provider page is missing its required reading count",
	"protocol/invalid_count":           "Provider reading count is null, negative or not an integer",
	"protocol/count_budget":            "Provider reading count exceeds the bounded record budget",
	"protocol/count_changed":           "Provider reading count changed between pages",
	"validation/invalid_decimal":       "Reading energy is not a supported nonnegative exact decimal",
	"validation/invalid_timestamp":     "Reading interval timestamp is invalid",
	"validation/invalid_duration":      "Reading interval duration is not 30 minutes",
	"validation/unaligned_interval":    "Reading start is not aligned to an absolute half-hour",
	"validation/outside_window":        "Reading is outside the requested window and is not an adjacent boundary neighbor",
	"validation/boundary_overlap":      "Reading straddles a requested window boundary",
	"validation/future_interval":       "Reading ends after the latest elapsed measurement time",
	"validation/conflicting_interval":  "Conflicting readings have the same interval start",
	"protocol/count_mismatch":          "Assembled reading count does not match the reported page count",
	"protocol/missing_results":         "Provider page has no required results array",
}

func validateDiagnostic(d Diagnostic) error {
	if !validDate(d.Period) {
		return ErrInvalid
	}
	for _, instant := range []*Instant{d.IntervalStart, d.IntervalEnd} {
		if instant != nil && !validInstant(*instant) {
			return ErrInvalid
		}
	}
	if d.Field != "" {
		valid := false
		for _, f := range []string{"soc", "bmsSOC", "ppv", "plocalLoadTotal", "pacToUserTotal", "pacToGridTotal", "pcharge1", "pdischarge1", "epvTotal", "epvtoday", "epv1Today", "epv2Today", "etoUserToday", "etoUserTotal", "etoGridToday", "etogridTotal", "echarge1Today", "echarge1Total", "edischarge1Today", "edischarge1Total", "elocalLoadToday", "elocalLoadTotal", "eacToday", "eacTotal"} {
			if d.Field == f {
				valid = true
			}
		}
		if !valid {
			return ErrInvalid
		}
	}
	if d.Value != "" && (len(d.Value) > 128 || !signedDecimal.MatchString(d.Value)) {
		return ErrInvalid
	}
	for _, n := range []*int{d.Expected, d.Observed} {
		if n != nil && *n < 0 {
			return ErrInvalid
		}
	}
	if d.HTTPStatus < 0 || d.HTTPStatus > 599 {
		return ErrInvalid
	}
	return nil
}
