package collection

import (
	"errors"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"math/rand/v2"
	"time"
)

func discoveryRetry(err error, now time.Time) (time.Duration, bool) {
	class := ""
	var deferred time.Time
	var ge *growatt.Error
	var oe *octopus.Error
	if errors.As(err, &ge) {
		class = ge.Class
		deferred = ge.DeferredUntil
	}
	if errors.As(err, &oe) {
		class = oe.Class
		deferred = oe.DeferredUntil
	}
	if class == "authentication" || class == "configuration" {
		return 0, true
	}
	delay := 15 * time.Minute
	if deferred.Sub(now) > delay {
		delay = deferred.Sub(now)
	}
	return delay, false
}

// Clients own inline attempts. The scheduler owns only a later durable attempt.
func retryDue(job storage.Job, now, provider time.Time) time.Time {
	n := job.RetryCount
	if n > 6 {
		n = 6
	}
	delay := time.Minute * time.Duration(1<<n)
	if delay > time.Hour {
		delay = time.Hour
	}
	due := now.Add(delay + time.Duration(rand.IntN(1001))*time.Millisecond)
	if provider.After(due) {
		due = provider
	}
	return due
}
