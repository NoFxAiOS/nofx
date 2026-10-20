package paper

import (
	"fmt"
	"time"

	"nofx/logger"
	"nofx/store"
	"nofx/trader/types"
)

func (p *PaperFuturesTrader) bind(traderID, exchangeID, exchangeType string, st *store.Store) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initErr != nil {
		return p.initErr
	}
	if exchangeID != p.cfg.ExchangeID || traderID == "" {
		return fmt.Errorf("paper sync identity must match the configured account")
	}
	if st != nil && st != p.st {
		return fmt.Errorf("paper sync cannot switch stores")
	}
	if exchangeType == "" {
		exchangeType = "paper"
	}
	if p.row.TraderID != "" && p.row.TraderID != traderID {
		return fmt.Errorf("paper account is already bound to trader %s", p.row.TraderID)
	}
	if p.row.TraderID == traderID && p.row.ExchangeType == exchangeType {
		return nil
	}
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		p.row.TraderID = traderID
		p.row.ExchangeType = exchangeType
		return nil, nil
	})
}

// SyncOrdersFromBinance preserves AutoTrader's optional lifecycle contract;
// there are no authenticated or Binance order-history calls here.
func (p *PaperFuturesTrader) SyncOrdersFromBinance(traderID, exchangeID, exchangeType string, st *store.Store) error {
	if err := p.bind(traderID, exchangeID, exchangeType, st); err != nil {
		return err
	}
	return p.Advance(p.cfg.Now())
}

func (p *PaperFuturesTrader) StartOrderSync(traderID, exchangeID, exchangeType string, st *store.Store, interval time.Duration) {
	if err := p.bind(traderID, exchangeID, exchangeType, st); err != nil {
		logger.Errorf("paper order sync: %v", err)
		return
	}
	if interval <= 0 || interval > 10*time.Second {
		interval = 10 * time.Second
	}
	p.mu.Lock()
	if p.stop != nil {
		p.mu.Unlock()
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	p.stop, p.done = stop, done
	p.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		run := func() {
			if err := p.Advance(p.cfg.Now()); err != nil {
				p.mu.Lock()
				name := p.displayName
				p.mu.Unlock()
				logger.Errorf("paper order sync [%s]: %v", name, err)
			}
		}
		types.RunSyncSafely(run)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				types.RunSyncSafely(run)
			}
		}
	}()
}

// StopOrderSync joins outside the state mutex, including concurrent stoppers.
func (p *PaperFuturesTrader) StopOrderSync() {
	p.mu.Lock()
	stop, done := p.stop, p.done
	if stop == nil {
		p.mu.Unlock()
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	p.mu.Unlock()
	<-done
	p.mu.Lock()
	if p.done == done {
		p.stop, p.done = nil, nil
	}
	p.mu.Unlock()
}
