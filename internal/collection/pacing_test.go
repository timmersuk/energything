package collection

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestEveryTelemetryTransmissionPacedAndCancellation(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	calls := 0
	waited := time.Duration(0)
	p := &pacedGrowatt{now: func() time.Time { return now }, wait: func(ctx context.Context, d time.Duration) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		waited += d
		now = now.Add(d)
		return nil
	}, base: transport(func(*http.Request) (*http.Response, error) { calls++; return &http.Response{StatusCode: 200}, nil })}
	request, _ := http.NewRequest("POST", "https://openapi.growatt.com/v1/device/mix/mix_data", nil)
	for n := 0; n < 3; n++ {
		if _, e := p.RoundTrip(request); e != nil {
			t.Fatal(e)
		}
	}
	if calls != 3 || waited != 20*time.Second {
		t.Fatal(calls, waited)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.RoundTrip(request.WithContext(ctx)); e == nil || calls != 3 {
		t.Fatal("cancelled pacing transmitted", e, calls)
	}
}
