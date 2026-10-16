package octopus

import (
	"github.com/timmersuk/energything/internal/storage"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var numeric = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var timestamp = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Expand valid JSON decimal exponents exactly, without a floating-point round trip.
func exact(raw string) (string, bool) {
	if len(raw) > 256 || !numeric.MatchString(raw) {
		return "", false
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == 'e' || r == 'E' })
	exponent := 0
	if len(parts) == 2 {
		var err error
		exponent, err = strconv.Atoi(parts[1])
		if err != nil || exponent > 256 || exponent < -256 {
			return "", false
		}
	}
	decimal := strings.IndexByte(parts[0], '.')
	if decimal < 0 {
		decimal = len(parts[0])
	}
	digits := strings.ReplaceAll(parts[0], ".", "")
	position := decimal + exponent
	var value string
	switch {
	case position <= 0:
		value = "0." + strings.Repeat("0", -position) + digits
	case position >= len(digits):
		value = digits + strings.Repeat("0", position-len(digits))
	default:
		value = digits[:position] + "." + digits[position:]
	}
	value = strings.TrimLeft(value, "0")
	if value == "" {
		value = "0"
	}
	if value[0] == '.' {
		value = "0" + value
	}
	if len(value) > 128 {
		return "", false
	}
	return value, true
}
func Validate(w *Window, from, to, now time.Time) error {
	if !validBounds(from, to) {
		return problem("configuration", "invalid", 0)
	}
	accepted := map[int64]Interval{}
	for _, r := range w.Readings {
		value, ok := exact(string(r.Energy))
		start, e1 := time.Parse(time.RFC3339, r.Start)
		end, e2 := time.Parse(time.RFC3339, r.End)
		invalid := func(code string) *Error {
			e := problem("validation", code, r.Evidence+1)
			if e1 == nil && start.UTC().Year() >= 1 && start.UTC().Year() <= 9999 {
				t := storage.At(start)
				e.Diagnostic.IntervalStart = &t
			}
			if e2 == nil && end.UTC().Year() >= 1 && end.UTC().Year() <= 9999 {
				t := storage.At(end)
				e.Diagnostic.IntervalEnd = &t
			}
			return e
		}
		if !ok {
			return invalid("invalid_decimal")
		}
		if !timestamp.MatchString(r.Start) || !timestamp.MatchString(r.End) || e1 != nil || e2 != nil {
			return invalid("invalid_timestamp")
		}
		if end.Sub(start) != Slot {
			return invalid("invalid_duration")
		}
		if start.Unix()%1800 != 0 || start.Nanosecond() != 0 {
			return invalid("unaligned_interval")
		}
		if !end.After(from) || !start.Before(to) {
			if end.Equal(from) || start.Equal(to) {
				w.Outside++
				continue
			}
			return invalid("outside_window")
		}
		if start.Before(from) || end.After(to) {
			return invalid("boundary_overlap")
		}
		if end.After(now) {
			return invalid("future_interval")
		}
		interval := Interval{start.UTC(), end.UTC(), value, r.Evidence}
		if prior, exists := accepted[start.Unix()]; exists {
			a, _ := new(big.Rat).SetString(prior.KWh)
			b, _ := new(big.Rat).SetString(value)
			if !prior.End.Equal(end) || a.Cmp(b) != 0 {
				return invalid("conflicting_interval")
			}
			w.Duplicates++
			continue
		}
		accepted[start.Unix()] = interval
	}
	w.Intervals = []Interval{}
	for _, r := range accepted {
		w.Intervals = append(w.Intervals, r)
	}
	sort.Slice(w.Intervals, func(i, j int) bool { return w.Intervals[i].Start.Before(w.Intervals[j].Start) })
	elapsed := to
	if now.Before(elapsed) {
		elapsed = now
	}
	if elapsed.Before(from) {
		elapsed = from
	}
	w.Expected = int(elapsed.Sub(from) / Slot)
	validElapsed := 0
	for _, r := range w.Intervals {
		if !r.End.After(elapsed) {
			validElapsed++
		}
	}
	w.Missing = w.Expected - validElapsed
	return nil
}
