// Package syncloop runs background exchange order-sync loops with a shared
// lifecycle: loops stop when the owning trader stops, and consecutive
// failures back off exponentially so a rate-limiting exchange (HTTP 429)
// is not hammered at full frequency.
package syncloop

import (
	"time"

	"nofx/logger"
)

// maxBackoff caps the failure backoff so a recovered exchange is picked up
// within a few minutes at worst.
const maxBackoff = 5 * time.Minute

// Run starts a background loop calling syncFn at the given interval until
// stop is closed. After each consecutive failure the wait doubles (capped at
// maxBackoff); the first success resets it to the base interval.
func Run(stop <-chan struct{}, interval time.Duration, name string, syncFn func() error) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		wait := interval
		timer := time.NewTimer(wait)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				logger.Infof("⏹ %s order sync stopped", name)
				return
			case <-timer.C:
				if err := syncFn(); err != nil {
					wait *= 2
					if wait > maxBackoff {
						wait = maxBackoff
					}
					logger.Infof("⚠️ %s order sync failed: %v (backing off, next attempt in %v)", name, err, wait)
				} else {
					wait = interval
				}
				timer.Reset(wait)
			}
		}
	}()
	logger.Infof("🔄 %s order sync started (interval: %v)", name, interval)
}
