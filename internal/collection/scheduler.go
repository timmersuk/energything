package collection

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"net/http"
	"sync"
	"time"
)

type Scheduler struct {
	DB      *storage.Store
	Config  config.Config
	Octopus *octopus.Client
	Growatt *growatt.Client
	Now     func() time.Time
	Wait    func(context.Context, time.Duration) error
}

func NewScheduler(db *storage.Store, c config.Config) *Scheduler {
	s := &Scheduler{DB: db, Config: c, Octopus: octopus.NewClient(c.Octopus.APIKey, nil), Now: time.Now, Wait: wait}
	s.Growatt = growatt.NewClient(c.Growatt.Token, &pacedGrowatt{now: func() time.Time { return s.Now() }, wait: func(ctx context.Context, d time.Duration) error { return s.Wait(ctx, d) }, base: http.DefaultTransport})
	return s
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Run starts exactly one independent lane per configured provider and joins both.
// Discovery/fetch errors stay in durable status; no network error stops browsing.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, provider := range []string{"octopus", "growatt"} {
		p := s.Config.Octopus
		if provider == "growatt" {
			p = s.Config.Growatt
		}
		if p.State != "valid" {
			continue
		}
		wg.Add(1)
		go func(provider string) { defer wg.Done(); s.lane(ctx, provider) }(provider)
	}
	wg.Wait()
}

func (s *Scheduler) discover(ctx context.Context, provider string) error {
	if provider == "growatt" {
		d, _, e := DiscoverGrowatt(ctx, s.DB, s.Growatt, s.Config.Growatt.PlantID, s.Config.Growatt.DeviceSerial)
		if d.SelectedPlant != "" {
			_, se := s.DB.SelectGeneration(ctx, d.SelectedPlant)
			if se != nil {
				return se
			}
			return e
		}
		if e != nil {
			return e
		}
		_, e = s.DB.SelectGeneration(ctx, "")
		return e
	}
	return DiscoverOctopus(ctx, s.DB, s.Octopus, s.Config.Octopus)
}

func (s *Scheduler) lane(ctx context.Context, provider string) {
	discoveryPending := true
	nextDiscovery := s.Now()
	if states, e := s.DB.QueryDiscovery(ctx); e == nil {
		for _, state := range states {
			if state.Provider == provider && state.Failure != nil && state.Failure.DeferredUntil != nil {
				due := time.UnixMicro(int64(*state.Failure.DeferredUntil))
				if due.After(nextDiscovery) {
					nextDiscovery = due
				}
			}
		}
	}
	resumedStartup := false
	for ctx.Err() == nil {
		if discoveryPending && !s.Now().Before(nextDiscovery) {
			e := s.discover(ctx, provider)
			if !resumedStartup {
				if s.DB.ResumeDiscoveredJobs(ctx, provider, storage.At(s.Now())) != nil {
					return
				}
				resumedStartup = true
			}
			if e == nil {
				discoveryPending = false
			} else {
				delay, paused := discoveryRetry(e, s.Now())
				discoveryPending = !paused
				nextDiscovery = s.Now().Add(delay)
			}
		}
		if provider == "octopus" {
			if s.probeBoundaries(ctx) != nil {
				if s.Wait(ctx, time.Minute) != nil {
					return
				}
				continue
			}
		}
		if e := s.prepare(ctx, provider, "", false); e != nil {
			if s.Wait(ctx, time.Minute) != nil {
				return
			}
			continue
		}
		jobs, e := s.DB.ScheduledJobs(ctx, storage.At(s.Now()))
		if e != nil {
			if s.Wait(ctx, time.Minute) != nil {
				return
			}
			continue
		}
		identities, e := s.DB.Identities(ctx)
		if e != nil {
			return
		}
		byID := map[int64]storage.Identity{}
		for _, i := range identities {
			byID[i.ID] = i
		}
		found := false
		for _, j := range jobs {
			i := byID[j.IdentityID]
			if i.Provider != provider {
				continue
			}
			found = true
			if provider == "octopus" {
				_ = CollectOctopusWindow(ctx, s.DB, s.Octopus, j, octopus.Meter{MPAN: i.Metadata.MPAN, Serial: i.Metadata.Serial, Direction: i.Stream})
			} else if j.Kind == "aggregate" {
				_ = CollectGrowattAggregateUnit(ctx, s.DB, s.Growatt, j, i.Metadata.PlantID)
			} else {
				_ = CollectGrowattUnit(ctx, s.DB, s.Growatt, j, growatt.Device{PlantID: i.Metadata.PlantID, Serial: i.Metadata.Serial, Family: i.Metadata.DeviceType}, s.Config.Timezone.String())
			}
			break // Re-arbitrate after every bounded unit.
		}
		delay := time.Second
		if !found {
			delay = 5 * time.Second
		}
		if provider == "growatt" && found {
			delay = 10 * time.Second
		}
		if s.Wait(ctx, delay) != nil {
			return
		}
	}
}

func (s *Scheduler) prepare(ctx context.Context, provider, stream string, force bool) error {
	ids, e := s.DB.Identities(ctx)
	if e != nil {
		return e
	}
	for _, i := range ids {
		if !i.Selected || i.Provider != provider || stream != "" && i.Stream != stream {
			continue
		}
		for _, p := range plans(i, s.Config, s.Now()) {
			if force && p.Policy != "routine" {
				continue
			}
			if i.Provider == "octopus" && p.Policy == "backfill" {
				b, be := s.DB.HistoryBoundary(ctx, i.ID)
				if be == nil && b.Oldest != nil {
					p.SearchPolicy = "bounded"
					p.SearchLimit = *b.Oldest
					p.Provenance = b.Provenance
				}
			}
			_, e = s.DB.PrepareSchedule(ctx, p, storage.At(s.Now()), force, func(saved storage.Schedule) storage.Job { return plannedWindow(saved, i, s.Now(), s.Config.Timezone) })
			if e != nil {
				return e
			}
		}
	}
	return nil
}

// Refresh only enqueues local routine work; cloud execution belongs to the lane.
func (s *Scheduler) Refresh(ctx context.Context, provider, stream string) ([]int64, error) {
	p := s.Config.Octopus
	if provider == "growatt" {
		p = s.Config.Growatt
	}
	if p.State != "valid" {
		return nil, storage.ErrInvalid
	}
	if e := s.prepare(ctx, provider, stream, true); e != nil {
		return nil, e
	}
	schedules, e := s.DB.QuerySchedules(ctx)
	if e != nil {
		return nil, e
	}
	ids, e := s.DB.Identities(ctx)
	if e != nil {
		return nil, e
	}
	selected := map[int64]bool{}
	for _, i := range ids {
		selected[i.ID] = i.Selected && i.Provider == provider && (stream == "" || i.Stream == stream)
	}
	out := []int64{}
	for _, p := range schedules {
		if selected[p.IdentityID] && p.Policy == "routine" && p.CurrentJob != nil {
			out = append(out, *p.CurrentJob)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("selected identity unavailable")
	}
	return out, nil
}
