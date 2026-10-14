package storage

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

// Instant is UTC Unix microseconds; source text is never converted implicitly.
type Instant int64

func At(t time.Time) Instant { return Instant(t.UnixMicro()) }

var ErrInvalid = errors.New("invalid storage request")
var ErrConflict = errors.New("storage state conflict")
var ErrNotFound = errors.New("storage record not found")
var decimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var signedDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

type DiscoveryMetadata struct {
	MPAN       string `json:"mpan,omitempty"`
	Serial     string `json:"serial,omitempty"`
	PlantID    string `json:"plant_id,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
}
type Identity struct {
	ID                          int64
	Provider, Stream, StableKey string
	Selected                    bool
	Metadata                    DiscoveryMetadata
}
type Job struct {
	ID, IdentityID          int64
	Kind, Policy            string
	Start, End              Instant
	ProviderDate, RequestID string
	NextDue                 Instant
	Checkpoint              *Instant
	RetryCount              int
	Paused                  bool
	RunningAttempt          *int64
}
type AttemptRequest struct {
	JobID   int64
	Started Instant
}
type Evidence struct {
	Content   json.RawMessage
	Retrieved Instant
	Complete  bool
}
type Interval struct {
	IdentityID  int64
	Start, End  Instant
	KWh         string
	Retrieved   Instant
	RetrievalID int64
	Evidence    int `json:"-"`
}
type Value struct {
	Decimal *string `json:"value"`
	Unit    *string `json:"unit"`
	Quality string  `json:"quality"`
}
type Sample struct {
	ID, IdentityID int64
	ResolvedKey    *string
	Instant        *Instant
	ProviderDate   string
	OriginalTime   map[string]string
	Quality        string
	Values         map[string]Value
	Retrieved      Instant
	RetrievalID    int64
	Evidence       int `json:"-"`
}
type Coverage struct {
	Expected, Valid, Missing, Qualified int
	MeasurementComplete                 bool
}
type AcceptedWindow struct {
	AttemptID         int64
	Finished, NextDue Instant
	Evidence          []Evidence
	Intervals         []Interval
	Samples           []Sample
	Retries           []RetryDetail
	TimestampQuality  bool
	// Growatt coverage must come from its protocol gate, not sample count guesses.
	Coverage Coverage
}
type RetryDetail struct {
	Page, Retry, HTTPStatus int
	Code                    string
	WaitMicroseconds        int64
}
type FailedAttempt struct {
	AttemptID                      int64
	Finished, NextDue              Instant
	Outcome, ErrorClass, ErrorCode string
	Page                           *int
	Paused                         bool
	Evidence                       []Evidence
	Retries                        []RetryDetail
}
type EvidenceRef struct {
	RetrievalID, EvidenceID int64
	Digest                  string
	Retrieved               Instant
	Complete                bool
}
type Audit struct {
	ID, JobID, IdentityID                   int64
	Provider, Stream, DisplayIdentity, Kind string
	Start, End                              Instant
	ProviderDate                            string
	Started, Occurred                       Instant
	Finished                                *Instant
	Outcome, ErrorClass, ErrorCode, Message string
	Page                                    *int
	RetryCount                              int
	NextRetry                               *Instant
	Retries                                 []RetryDetail
	AcceptedRows                            int
	Evidence                                []EvidenceRef
}
type WindowStatus struct {
	JobID, AttemptID int64
	Start, End       Instant
	Fetched          bool
	Coverage         Coverage
}
type Status struct {
	Metadata                                    DiscoveryMetadata `json:"-"`
	IdentityID                                  int64
	Provider, Stream                            string
	Selected                                    bool
	IntervalCount, SampleCount, UnresolvedCount int64
	LastRetrieval, NewestMeasurement            *Instant
	LastAttempt                                 *Audit
	LastAcceptedAttempt                         *Audit
	JobCount, WindowCount                       int64
	// Detail lists are capped at 500; counts cover all retained work.
	Jobs    []Job
	Windows []WindowStatus
}

func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
func validText(s string) bool { return len(s) > 0 && len(s) <= 256 }

func validInstant(t Instant) bool { return t >= -62135596800000000 && t <= 253402300799999999 }
