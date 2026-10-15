package collection

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Each detailed/discovery transmission, including inline retry/page requests,
// observes a ten-second minimum. Aggregate requests use their durable ledger.
type pacedGrowatt struct {
	mu   sync.Mutex
	next time.Time
	now  func() time.Time
	wait func(context.Context, time.Duration) error
	base http.RoundTripper
}

func (p *pacedGrowatt) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/v1/plant/energy" {
		return p.base.RoundTrip(r)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if d := p.next.Sub(p.now()); d > 0 {
		if e := p.wait(r.Context(), d); e != nil {
			return nil, e
		}
	}
	if e := r.Context().Err(); e != nil {
		return nil, e
	}
	p.next = p.now().Add(10 * time.Second)
	return p.base.RoundTrip(r)
}
