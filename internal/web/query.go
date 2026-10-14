package web

import (
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/timmersuk/energything/internal/storage"
)

type recordQuery struct {
	storage.SampleQuery
	FromText, ToText string
}

// Both HTML and JSON use this parser. Calendar ends are inclusive; instant ends
// are exclusive. Quality-view bounds apply to retrieval, never guessed clock text.
func parseQuery(v url.Values, growatt bool, loc *time.Location, now time.Time) (q recordQuery, err error) {
	invalid := errors.New("invalid query")
	allowed := map[string]bool{"from": true, "to": true, "limit": true, "cursor": true, "identity": true}
	if growatt {
		allowed["view"] = true
		allowed["quality"] = true
		allowed["provider_date"] = true
	} else {
		allowed["direction"] = true
	}
	for k, values := range v {
		if !allowed[k] || len(values) != 1 {
			return q, invalid
		}
	}
	q.Limit = 100
	if v.Has("limit") {
		q.Limit, err = strconv.Atoi(v.Get("limit"))
		if err != nil || q.Limit < 1 || q.Limit > 500 {
			return q, invalid
		}
	}
	if v.Get("identity") != "" {
		q.IdentityID, err = strconv.ParseInt(v.Get("identity"), 10, 64)
		if err != nil || q.IdentityID <= 0 {
			return q, invalid
		}
	}
	q.Cursor = v.Get("cursor")
	if len(q.Cursor) > 1024 {
		return q, invalid
	}
	q.Direction = v.Get("direction")
	if q.Direction != "" && q.Direction != "import" && q.Direction != "export" {
		return q, invalid
	}
	switch v.Get("view") {
	case "", "resolved":
	case "quality":
		q.QualityView = true
	default:
		return q, invalid
	}
	q.Quality = v.Get("quality")
	q.ProviderDate = v.Get("provider_date")
	if (q.Quality != "" || q.ProviderDate != "") && !q.QualityView {
		return q, invalid
	}
	if len(q.Quality) > 256 {
		return q, invalid
	}
	if q.ProviderDate != "" {
		if _, err = time.Parse("2006-01-02", q.ProviderDate); err != nil {
			return q, invalid
		}
	}
	from, to := v.Get("from"), v.Get("to")
	if (from == "") != (to == "") {
		return q, invalid
	}
	// A provider-date quality view needs no retrieval restriction unless requested.
	if from == "" && q.QualityView && q.ProviderDate != "" {
		return q, nil
	}
	if from == "" {
		from = now.In(loc).Format("2006-01-02")
		to = from
	}
	q.FromText, q.ToText = from, to
	var a, b time.Time
	if len(from) == 10 && len(to) == 10 {
		a, err = time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			return q, invalid
		}
		b, err = time.ParseInLocation("2006-01-02", to, loc)
		if err != nil || b.Before(a) {
			return q, invalid
		}
		b = b.AddDate(0, 0, 1)
	} else {
		a, err = time.Parse(time.RFC3339Nano, from)
		if err != nil {
			return q, invalid
		}
		b, err = time.Parse(time.RFC3339Nano, to)
		if err != nil {
			return q, invalid
		}
	}
	if !b.After(a) || a.UTC().Year() < 1 || b.UTC().Year() > 9999 || a.Nanosecond()%1000 != 0 || b.Nanosecond()%1000 != 0 {
		return q, invalid
	}
	x, y := storage.At(a), storage.At(b)
	q.From, q.To = &x, &y
	return q, nil
}

func (q recordQuery) nextURL(path, cursor string) string {
	v := url.Values{"limit": {strconv.Itoa(q.Limit)}}
	if q.FromText != "" {
		v.Set("from", q.FromText)
		v.Set("to", q.ToText)
	}
	if q.IdentityID != 0 {
		v.Set("identity", strconv.FormatInt(q.IdentityID, 10))
	}
	if q.Direction != "" {
		v.Set("direction", q.Direction)
	}
	if q.QualityView {
		v.Set("view", "quality")
	}
	if q.Quality != "" {
		v.Set("quality", q.Quality)
	}
	if q.ProviderDate != "" {
		v.Set("provider_date", q.ProviderDate)
	}
	v.Set("cursor", cursor)
	return path + "?" + v.Encode()
}
