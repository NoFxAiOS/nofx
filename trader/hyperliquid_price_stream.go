package trader

import (
	"context"
	"fmt"
	"nofx/logger"
	"sync"
	"time"

	"github.com/sonirico/go-hyperliquid"
)

// hyperliquidPriceEvent carries a normalized symbol with the latest mid price
type hyperliquidPriceEvent struct {
	symbol string
	price  float64
	ts     time.Time
}

// HyperliquidPriceStream manages websocket subscriptions for price updates
type HyperliquidPriceStream struct {
	client *hyperliquid.WebsocketClient
	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	subs map[string]*hyperliquid.Subscription
}

// NewHyperliquidPriceStream creates a websocket client for mainnet/testnet
func NewHyperliquidPriceStream(testnet bool) *HyperliquidPriceStream {
	baseURL := hyperliquid.MainnetAPIURL
	if testnet {
		baseURL = hyperliquid.TestnetAPIURL
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &HyperliquidPriceStream{
		client: hyperliquid.NewWebsocketClient(baseURL),
		ctx:    ctx,
		cancel: cancel,
		subs:   make(map[string]*hyperliquid.Subscription),
	}
}

// Start connects the websocket client
func (s *HyperliquidPriceStream) Start() error {
	return s.ensureConnected()
}

// SubscribeBbo subscribes to best bid/offer for a coin and returns mid prices
func (s *HyperliquidPriceStream) SubscribeBbo(coin string, handler func(price float64, ts time.Time)) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	if err := s.ensureConnected(); err != nil {
		return err
	}

	s.mu.Lock()
	if _, ok := s.subs[coin]; ok {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	sub, err := s.client.Bbo(hyperliquid.BboSubscriptionParams{Coin: coin}, func(bbo hyperliquid.Bbo, err error) {
		if err != nil {
			logger.Infof("⚠️ Hyperliquid BBO error (%s): %v", coin, err)
			return
		}
		if len(bbo.Bbo) == 0 {
			return
		}

		var mid float64
		if len(bbo.Bbo) >= 2 {
			mid = (bbo.Bbo[0].Px + bbo.Bbo[1].Px) / 2
		} else {
			mid = bbo.Bbo[0].Px
		}

		handler(mid, time.UnixMilli(bbo.Time))
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.subs[coin] = sub
	s.mu.Unlock()

	return nil
}

// Unsubscribe removes a subscription for the given coin
func (s *HyperliquidPriceStream) Unsubscribe(coin string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sub, ok := s.subs[coin]; ok && sub != nil && sub.Close != nil {
		sub.Close()
	}
	delete(s.subs, coin)
}

// Close shuts down all subscriptions and the underlying websocket
func (s *HyperliquidPriceStream) Close() {
	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	for coin, sub := range s.subs {
		if sub != nil && sub.Close != nil {
			sub.Close()
		}
		delete(s.subs, coin)
	}
}

// ensureConnected establishes websocket connection if not already connected
func (s *HyperliquidPriceStream) ensureConnected() error {
	if s.client == nil {
		return fmt.Errorf("nil websocket client")
	}
	return s.client.Connect(s.ctx)
}
