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
