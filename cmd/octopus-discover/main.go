// octopus-discover is a read-only, private meter-selection report. It never
// writes configuration, accepted observations or the application's database.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/timmersuk/energything/internal/octopus"
)

type source interface {
	Account(context.Context, string) (octopus.Account, error)
	Fetch(context.Context, octopus.Meter, time.Time, time.Time) (octopus.Window, error)
}
type candidate struct {
	Meter                           octopus.Meter
	State, ErrorClass, ErrorCode    string
	Valid, Expected, Pages, Retries int
	First, Last                     time.Time
}
type streamReport struct {
	Direction, Discovery string
	Candidates           []candidate
	Suggested            *octopus.Meter
	Reason               string
}
type report struct {
	Generated, From, To time.Time
	Streams             []streamReport
	Unknown             int
	Complete            bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, get func(string) string, out io.Writer) error {
	flags := flag.NewFlagSet("octopus-discover", flag.ContinueOnError)
	// Flag parser diagnostics can echo supplied text. Keep errors sanitary.
	flags.SetOutput(io.Discard)
	days := flags.Int("days", 14, "recent absolute days to sample (1–14)")
	timeout := flags.Duration("timeout", 2*time.Minute, "total discovery and sampling deadline (up to 10m)")
	output := flags.String("out", "", "private Markdown report path; default is a new file under .cache/octopus-discovery")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: octopus-discover [--days 1–14] [--timeout 2m] [--out private-report.md]. Reads OCTOPUS_API_KEY and OCTOPUS_ACCOUNT_NUMBER from process environment. Defaults to a private report under .cache/octopus-discovery; never changes selections or the database.")
			return err
		}
		return errors.New("invalid options; use --days 1–14, --timeout 2m, --out private-path, or --help")
	}
	if flags.NArg() != 0 || *days < 1 || *days > 14 || *timeout <= 0 || *timeout > 10*time.Minute {
		return errors.New("invalid options: days must be 1–14 and timeout positive and at most 10m")
	}
	key, account := get("OCTOPUS_API_KEY"), get("OCTOPUS_ACCOUNT_NUMBER")
	if key == "" || account == "" {
		return errors.New("set OCTOPUS_API_KEY and OCTOPUS_ACCOUNT_NUMBER in the process environment; no .env file is loaded")
	}
	deadline, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	to := time.Now().UTC().Truncate(octopus.Slot)
	from := to.Add(-time.Duration(*days) * 24 * time.Hour)
	result, err := investigate(deadline, octopus.NewClient(key, nil), account, from, to)
	if err != nil {
		return err
	}
	name, err := writeReport(*output, result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Private discovery report: %s\nSampling complete: %t. No selections or database records were changed.\n", name, result.Complete)
	return err
}

func investigate(ctx context.Context, client source, account string, from, to time.Time) (report, error) {
	result := report{Generated: time.Now().UTC(), From: from, To: to, Complete: true}
	a, err := client.Account(ctx, account)
	if err != nil {
		var e *octopus.Error
		if errors.As(err, &e) {
			return result, fmt.Errorf("account discovery failed: %s/%s", e.Class, e.Code)
		}
		return result, errors.New("account discovery failed")
	}
	discovery := octopus.Select(a, octopus.Choice{}, octopus.Choice{})
	for _, property := range a.Properties {
		for _, point := range property.Points {
			if point.IsExport == nil {
				result.Unknown += len(point.Meters)
			}
		}
	}
	total := len(discovery.Import.Candidates) + len(discovery.Export.Candidates)
	// One bounded sequential lane, with a total deadline and no credential storms.
	stopped := ""
	if total > 20 {
		stopped = "candidate_budget"
		result.Complete = false
	}
	for _, item := range []struct {
		direction string
		stream    octopus.Stream
	}{{"import", discovery.Import}, {"export", discovery.Export}} {
		s := streamReport{Direction: item.direction, Discovery: item.stream.State}
		for _, meter := range item.stream.Candidates {
			c := candidate{Meter: meter, State: "not_sampled"}
			switch {
			case item.stream.State == "conflict":
				c.ErrorClass = "selection"
				c.ErrorCode = "conflict"
			case stopped != "":
				c.ErrorClass = "sampling"
				c.ErrorCode = stopped
			case ctx.Err() != nil:
				c.ErrorClass = "network"
				c.ErrorCode = "cancelled"
				stopped = "cancelled"
			default:
				window, fetchErr := client.Fetch(ctx, meter, from, to)
				c.Pages = len(window.Evidence)
				c.Retries = len(window.Retries)
				if fetchErr != nil {
					c.State = "failed"
					c.ErrorClass, c.ErrorCode = "protocol", "invalid"
					var e *octopus.Error
					if errors.As(fetchErr, &e) {
						c.ErrorClass, c.ErrorCode = e.Class, e.Code
						if e.Class == "authentication" || e.Class == "rate_limit" || e.Code == "cancelled" || !e.DeferredUntil.IsZero() {
							stopped = e.Class + "/" + e.Code
						}
					}
				} else {
					c.State = "sampled"
					c.Valid = len(window.Intervals)
					c.Expected = window.Expected
					for _, r := range window.Intervals {
						if c.First.IsZero() || r.Start.Before(c.First) {
							c.First = r.Start
						}
						if c.Last.IsZero() || r.End.After(c.Last) {
							c.Last = r.End
						}
					}
				}
			}
			if c.State != "sampled" {
				result.Complete = false
			}
			s.Candidates = append(s.Candidates, c)
		}
		assess(&s, to)
		result.Streams = append(result.Streams, s)
	}
	return result, nil
}
func assess(s *streamReport, to time.Time) {
	// Ranking is descriptive only: timestamps then coverage. Never use energy size
	// to infer a household meter, and zero-energy intervals still count as telemetry.
	sort.SliceStable(s.Candidates, func(i, j int) bool {
		a, b := s.Candidates[i], s.Candidates[j]
		if !a.Last.Equal(b.Last) {
			return a.Last.After(b.Last)
		}
		if a.Valid != b.Valid {
			return a.Valid > b.Valid
		}
		return a.Meter.Key() < b.Meter.Key()
	})
	if len(s.Candidates) == 0 {
		s.Reason = "No confirmed candidates discovered."
		return
	}
	if s.Discovery == "conflict" {
		s.Reason = "Conflicting discovery descriptions block this direction; no candidates were sampled."
		return
	}
	for _, c := range s.Candidates {
		if c.State != "sampled" {
			s.Reason = "Sampling is incomplete or failed. Unsampled/failed candidates cannot be treated as inactive; no selection is suggested."
			return
		}
	}
	if len(s.Candidates) == 1 {
		m := s.Candidates[0].Meter
		s.Suggested = &m
		s.Reason = "Only one confirmed identity was discovered. The application can auto-select it; recent data may still be empty."
		return
	}
	recent := []candidate{}
	for _, c := range s.Candidates {
		if !c.Last.IsZero() && !c.Last.Before(to.Add(-48*time.Hour)) {
			recent = append(recent, c)
		}
	}
	if len(recent) == 1 {
		m := recent[0].Meter
		s.Suggested = &m
		s.Reason = "Tentative steer: only this candidate has validated intervals ending within the last 48 hours of the sample. Confirm it against the physical meter/account before applying."
		return
	}
	if len(recent) > 1 {
		s.Reason = "Several candidates report recent intervals. Recency and coverage are shown, but do not resolve the choice; no selection is suggested."
		return
	}
	s.Reason = "No candidate reports an interval ending within the last 48 hours of the sample. An empty/gappy response does not establish a meter's lifetime; no selection is suggested."
}
func render(out io.Writer, r report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Private Octopus meter discovery report\n\nGenerated: %s\n\nSample: %s to %s (exclusive), UTC.\n\nSampling complete: %t. Unknown-direction meter entries skipped: %d.\n\n", r.Generated.Format(time.RFC3339), r.From.Format(time.RFC3339), r.To.Format(time.RFC3339), r.Complete, r.Unknown)
	b.WriteString("This report contains private meter identifiers. It contains no API keys, account/address payloads or individual consumption values. Sampling is read-only and does not populate the application database or change configuration. Missing readings do not mean zero energy or prove inactivity. Ranking uses newest validated interval end, then observed slot count; no automatic selection is made.\n\n")
	for _, s := range r.Streams {
		fmt.Fprintf(&b, "## %s\n\nDiscovery: %s\n\n| MPAN | Serial | Sampling | Valid / expected slots | First start UTC | Latest end UTC | Pages / retries |\n| --- | --- | --- | --- | --- | --- | --- |\n", s.Direction, s.Discovery)
		for _, c := range s.Candidates {
			state := c.State
			if c.ErrorCode != "" {
				state += " (" + c.ErrorClass + "/" + c.ErrorCode + ")"
			}
			coverage := "unknown"
			if c.State == "sampled" {
				coverage = fmt.Sprintf("%d / %d", c.Valid, c.Expected)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %d / %d |\n", c.Meter.MPAN, c.Meter.Serial, state, coverage, clock(c.First), clock(c.Last), c.Pages, c.Retries)
		}
		fmt.Fprintf(&b, "\n%s\n\n", s.Reason)
		if s.Suggested != nil {
			prefix := "OCTOPUS_" + strings.ToUpper(s.Direction)
			fmt.Fprintf(&b, "Candidate settings for your confirmation (not applied):\n\n```text\n%s_MPAN=%s\n%s_SERIAL=%s\n```\n\n", prefix, s.Suggested.MPAN, prefix, s.Suggested.Serial)
		}
	}
	b.WriteString("After confirming selections, configure the energything runtime privately and explicitly run its one-shot ingestion. This diagnostic alone does not ingest data. Provider destinations, validation and page/retry limits are the same as the application client.\n")
	_, err := io.WriteString(out, b.String())
	return err
}
func clock(t time.Time) string {
	if t.IsZero() {
		return "none"
	}
	return t.UTC().Format(time.RFC3339)
}
func writeReport(path string, r report) (string, error) {
	if path == "" {
		dir := filepath.Join(".cache", "octopus-discovery")
		if os.MkdirAll(dir, 0700) != nil {
			return "", errors.New("cannot create private report directory")
		}
		f, err := os.CreateTemp(dir, "report-*.md")
		if err != nil {
			return "", errors.New("cannot create private report")
		}
		return finishReport(f, r)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("invalid report destination")
	}
	if os.MkdirAll(filepath.Dir(absolute), 0700) != nil {
		return "", errors.New("cannot create private report directory")
	}
	f, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", errors.New("cannot create report; choose a new private output file (existing files are never overwritten)")
	}
	return finishReport(f, r)
}
func finishReport(f *os.File, r report) (string, error) {
	name := f.Name()
	err := render(f, r)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(name)
		return "", errors.New("private report write failed")
	}
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", errors.New("invalid report destination")
	}
	return absolute, nil
}
