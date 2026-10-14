package growatt

import (
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var numeric = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func exact(raw string) (string, bool) {
	if len(raw) > 256 || !numeric.MatchString(raw) {
		return "", false
	}
	negative := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == 'e' || r == 'E' })
	exponent := 0
	if len(parts) == 2 {
		n, e := strconv.Atoi(parts[1])
		if e != nil || n > 256 || n < -256 {
			return "", false
		}
		exponent = n
	}
	point := strings.IndexByte(parts[0], '.')
	if point < 0 {
		point = len(parts[0])
	}
	digits := strings.ReplaceAll(parts[0], ".", "")
	position := point + exponent
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
	if negative {
		value = "-" + value
	}
	if len(value) > 128 {
		return "", false
	}
	return value, true
}

var fields = []string{"soc", "bmsSOC", "ppv", "plocalLoadTotal", "pacToUserTotal", "pacToGridTotal", "pcharge1", "pdischarge1", "epvTotal", "epvtoday", "epv1Today", "epv2Today", "etoUserToday", "etoUserTotal", "etoGridToday", "etogridTotal", "echarge1Today", "echarge1Total", "edischarge1Today", "edischarge1Total", "elocalLoadToday", "elocalLoadTotal", "eacToday", "eacTotal"}

func normalize(row map[string]any, date, serial string, evidence int) (storage.Sample, error) {
	s := storage.Sample{ProviderDate: date, OriginalTime: map[string]string{}, Quality: "unresolved_time", Values: map[string]storage.Value{}, Evidence: evidence}
	if v, ok := row["serialNum"]; ok && v != nil && text(v) != serial {
		return s, problem("validation", "identity_mismatch", evidence+1)
	}
	for _, key := range []string{"time", "utcTime", "createTime", "calendar"} {
		if v, ok := row[key]; ok {
			b, _ := json.Marshal(v)
			s.OriginalTime[key] = string(b)
		}
	}
	clock := text(row["time"])
	if clock == "" {
		s.Quality = "missing_time"
	} else if t, e := time.Parse("2006-01-02 15:04:05", clock); e == nil {
		if date != "" && t.Format("2006-01-02") != date {
			return s, problem("validation", "sample_date", evidence+1)
		}
		s.ProviderDate = t.Format("2006-01-02")
		if calendar, ok := row["calendar"].(map[string]any); ok {
			if java, ok := calendar["time"].(map[string]any); ok {
				h, hok := integer(java["hours"])
				day, dok := integer(java["date"])
				if hok && dok && (h != t.Hour() || day != t.Day()) {
					s.Quality = "contradictory_time"
				}
			}
		}
	}
	for _, key := range fields {
		v := storage.Value{Quality: "missing"}
		raw, ok := row[key]
		if !ok || raw == nil {
			s.Values[key] = v
			continue
		}
		value, ok := exact(text(raw))
		if !ok {
			pe := problem("validation", "sample_decimal", evidence+1)
			pe.Diagnostic.Field = key
			return s, pe
		}
		v.Decimal = &value
		v.Quality = "unit_unresolved"
		if key == "soc" || key == "bmsSOC" {
			r, _ := new(big.Rat).SetString(value)
			if r.Sign() < 0 || r.Cmp(big.NewRat(100, 1)) > 0 {
				pe := problem("validation", "soc_range", evidence+1)
				pe.Diagnostic.Field = key
				pe.Diagnostic.Value = value
				return s, pe
			}
			if key == "soc" {
				unit := "%"
				v.Unit = &unit
				v.Quality = "verified"
			}
		}
		if key == "ppv" {
			r, _ := new(big.Rat).SetString(value)
			if r.Sign() < 0 {
				return s, problem("validation", "pv_negative", evidence+1)
			}
			unit := "W"
			v.Unit = &unit
			v.Quality = "verified"
			if label, ok := row["ppvText"].(string); ok && label != "" {
				parts := strings.Fields(label)
				if len(parts) != 2 || parts[1] != "W" {
					return s, problem("validation", "pv_unit_conflict", evidence+1)
				}
				other, ok := exact(parts[0])
				if !ok {
					return s, problem("validation", "pv_unit_conflict", evidence+1)
				}
				q, _ := new(big.Rat).SetString(other)
				if r.Cmp(q) != 0 {
					return s, problem("validation", "pv_unit_conflict", evidence+1)
				}
			}
		}
		s.Values[key] = v
	}
	return s, nil
}
