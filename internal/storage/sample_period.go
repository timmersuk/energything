package storage

import (
	"encoding/json"
	"strings"
	"time"
)

// The provider's reported timestamp is the canonical period. Parsing validates
// its syntax only; it does not establish a timezone or measurement instant.
func samplePeriod(s Sample) (string, error) {
	var clock string
	if json.Unmarshal([]byte(s.OriginalTime["time"]), &clock) != nil || len(clock) != 19 || !strings.HasPrefix(clock, s.ProviderDate+" ") {
		return "", ErrInvalid
	}
	if _, err := time.Parse("2006-01-02 15:04:05", clock); err != nil {
		return "", ErrInvalid
	}
	return clock, nil
}

// Provider pages may overlap. Equal values for the same source period are one
// reading; disagreement within one window is a conflict, not a correction.
func canonicalSamples(samples []Sample) ([]Sample, error) {
	out := make([]Sample, 0, len(samples))
	positions := map[string]int{}
	for _, sample := range samples {
		period, err := samplePeriod(sample)
		if err != nil {
			return nil, err
		}
		if i, exists := positions[period]; exists {
			if !sameSampleValues(out[i].Values, sample.Values) {
				return nil, ErrInvalid
			}
			out[i] = sample
			continue
		}
		positions[period] = len(out)
		out = append(out, sample)
	}
	return out, nil
}

func sameSampleValues(a, b map[string]Value) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		other, ok := b[key]
		if !ok || value.Quality != other.Quality || (value.Unit == nil) != (other.Unit == nil) || (value.Decimal == nil) != (other.Decimal == nil) {
			return false
		}
		if value.Unit != nil && *value.Unit != *other.Unit {
			return false
		}
		if value.Decimal != nil && !sameDecimal(*value.Decimal, *other.Decimal) {
			return false
		}
	}
	return true
}
