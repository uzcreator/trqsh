package server

import (
	"context"
	"sync"
	"time"

	"github.com/rift/rift/pkg/authz"
)

// usageKey identifies a metering bucket.
type usageKey struct {
	accountID string
	tunnelID  string
}

// usageAgg aggregates per-account/per-tunnel traffic in short windows and
// flushes them to the entitlements plane (billing) via ReportUsage.
type usageAgg struct {
	ent   authz.Entitlements
	every time.Duration

	mu      sync.Mutex
	windows map[usageKey]*authz.Usage
	stop    chan struct{}
	wg      sync.WaitGroup
}

func newUsageAgg(ent authz.Entitlements, every time.Duration) *usageAgg {
	return &usageAgg{
		ent:     ent,
		every:   every,
		windows: make(map[usageKey]*authz.Usage),
		stop:    make(chan struct{}),
	}
}

// record adds traffic to the current window for an account/tunnel.
func (u *usageAgg) record(accountID, tunnelID string, bytesIn, bytesOut, requests int64) {
	if accountID == "" {
		return
	}
	k := usageKey{accountID: accountID, tunnelID: tunnelID}
	u.mu.Lock()
	defer u.mu.Unlock()
	w := u.windows[k]
	if w == nil {
		w = &authz.Usage{AccountID: accountID, TunnelID: tunnelID, WindowStart: time.Now()}
		u.windows[k] = w
	}
	w.BytesIn += bytesIn
	w.BytesOut += bytesOut
	w.Requests += requests
}

// run flushes accumulated windows on a ticker until Stop.
func (u *usageAgg) run() {
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		t := time.NewTicker(u.every)
		defer t.Stop()
		for {
			select {
			case <-u.stop:
				u.flush()
				return
			case <-t.C:
				u.flush()
			}
		}
	}()
}

func (u *usageAgg) flush() {
	u.mu.Lock()
	pending := u.windows
	u.windows = make(map[usageKey]*authz.Usage)
	u.mu.Unlock()

	now := time.Now()
	for _, w := range pending {
		if w.BytesIn == 0 && w.BytesOut == 0 && w.Requests == 0 {
			continue
		}
		w.WindowEnd = now
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = u.ent.ReportUsage(ctx, *w)
		cancel()
	}
}

// Stop flushes and stops the aggregator.
func (u *usageAgg) Stop() {
	close(u.stop)
	u.wg.Wait()
}
