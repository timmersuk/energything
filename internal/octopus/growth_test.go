package octopus

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestConsumptionCountGrowth(t *testing.T) {
	calls := 0
	c := client(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(200, page([]string{row(from, "1")}, nextURL(r), 2)), nil
		}
		return response(200, page([]string{row(from.Add(Slot), "2"), row(from.Add(2*Slot), "3")}, "", 3)), nil
	})
	w, err := c.Fetch(context.Background(), meter, from, from.Add(3*Slot))
	if err != nil || len(w.Intervals) != 3 || calls != 2 {
		t.Fatalf("growing consumption discarded: intervals=%d calls=%d err=%v", len(w.Intervals), calls, err)
	}
	if len(w.Evidence) != 2 || !w.Evidence[1].Complete {
		t.Fatal("missing completed evidence")
	}
}

func TestMalformedCountEvidenceExcludesPayload(t *testing.T) {
	c := client(func(*http.Request) (*http.Response, error) {
		return response(200, `{"count":{"message":"PRIVATE"},"next":null,"results":[]}`), nil
	})
	w, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
	if err == nil || len(w.Evidence) != 1 || strings.Contains(string(w.Evidence[0].Content), "PRIVATE") {
		t.Fatal("malformed count payload retained", err)
	}
}
