package collection

import (
	"github.com/timmersuk/energything/internal/storage"
	"math/rand/v2"
	"time"
)

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
