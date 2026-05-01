package market

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type oiSnapshot struct {
	Symbol         string
	OIUSD          float64
	VolumeUSD      float64
	PriceChangePct float64
	UpdatedAt      time.Time
}

type oiDeltaState struct {
	Snapshots map[string]oiSnapshot
	UpdatedAt time.Time
}

var oiDeltaStore sync.Map

func computeOIDeltaScores(exchange string, current []HotCoin, top bool) ([]HotCoin, bool) {
	now := time.Now().UTC()
	key := strings.ToLower(exchange)
	prevRaw, hasPrev := oiDeltaStore.Load(key)
	prev := map[string]oiSnapshot{}
	shouldAdvanceBaseline := false
	if hasPrev {
		prevState := prevRaw.(*oiDeltaState)
		if time.Since(prevState.UpdatedAt) <= 2*time.Hour {
			prev = prevState.Snapshots
			shouldAdvanceBaseline = time.Since(prevState.UpdatedAt) >= 180*time.Second
		}
	}
	latest := make(map[string]oiSnapshot, len(current))
	for _, c := range current {
		latest[c.Symbol] = oiSnapshot{Symbol: c.Symbol, OIUSD: c.OpenInterestUSD, VolumeUSD: c.QuoteVolume24h, PriceChangePct: c.PriceChangePct, UpdatedAt: now}
	}
	if len(prev) == 0 {
		oiDeltaStore.Store(key, &oiDeltaState{Snapshots: latest, UpdatedAt: now})
		return current, false
	}
	out := make([]HotCoin, 0, len(current))
	for _, c := range current {
		p, ok := prev[c.Symbol]
		if !ok || p.OIUSD <= 0 || c.OpenInterestUSD <= 0 {
			continue
		}
		deltaPct := ((c.OpenInterestUSD - p.OIUSD) / p.OIUSD) * 100
		c.OpenInterestChangePct = deltaPct
		c.OpenInterestWindowSec = int(now.Sub(p.UpdatedAt).Seconds())
		c.OpenInterestSource = "local_snapshot"
		if top && deltaPct <= 0 {
			continue
		}
		if !top && deltaPct >= 0 {
			continue
		}
		c.HotScore = scoreOIDeltaCandidate(deltaPct, c.Quality)
		if c.Quality.Reasons == nil {
			c.Quality.Reasons = []string{}
		}
		c.Quality.Reasons = append(c.Quality.Reasons, "oi_delta_snapshot")
		out = append(out, c)
	}
	if shouldAdvanceBaseline {
		oiDeltaStore.Store(key, &oiDeltaState{Snapshots: latest, UpdatedAt: now})
	}
	if top {
		sort.Slice(out, func(i, j int) bool { return out[i].HotScore > out[j].HotScore })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].HotScore < out[j].HotScore })
	}
	return out, true
}
