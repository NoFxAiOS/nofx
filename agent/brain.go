package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"nofx/safe"
	"strings"
	"sync"
	"time"
)

// Brain handles proactive intelligence: signals, news, market briefs.
type Brain struct {
	agent         *Agent
	logger        *slog.Logger
	http          *http.Client
	stopCh        chan struct{}
	stopOnce      sync.Once
	recentSignals sync.Map // debounce
}

func NewBrain(agent *Agent, logger *slog.Logger) *Brain {
	return &Brain{
		agent:  agent,
		logger: logger,
		http:   &http.Client{Timeout: 15 * time.Second},
		stopCh: make(chan struct{}),
	}
}

func (b *Brain) Stop() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
}

// cleanStaleSignals removes debounce entries older than 30 minutes.
func (b *Brain) cleanStaleSignals() {
	cutoff := time.Now().Add(-30 * time.Minute)
	b.recentSignals.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			b.recentSignals.Delete(key)
		}
		return true
	})
}

func (b *Brain) HandleSignal(sig Signal) {
	key := fmt.Sprintf("%s:%s", sig.Type, sig.Symbol)
	if v, ok := b.recentSignals.Load(key); ok {
		if time.Since(v.(time.Time)) < 10*time.Minute {
			return
		}
	}
	b.recentSignals.Store(key, time.Now())

	emoji := map[string]string{"info": "ℹ️", "warning": "⚠️", "critical": "🚨"}
	e := emoji[sig.Severity]
	if e == "" {
		e = "📊"
	}

	b.agent.notifyAll(fmt.Sprintf("%s *%s*\n\n%s", e, sig.Title, sig.Detail))
}

func (b *Brain) StartNewsScan(interval time.Duration) {
	seen := make(map[string]bool)
	seenOrder := make([]string, 0, 1024)
	safe.GoNamed("brain-news-scan", func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		cleanTick := 0
		for {
			select {
			case <-b.stopCh:
				return
			case <-ticker.C:
				b.scanNews(seen, &seenOrder)
				cleanTick++
				if cleanTick%6 == 0 { // every ~30 min
					b.cleanStaleSignals()
				}
			}
		}
	})
}

func (b *Brain) scanNews(seen map[string]bool, seenOrder *[]string) {
	resp, err := b.http.Get("https://min-api.cryptocompare.com/data/v2/news/?lang=EN&sortOrder=latest")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b.logger.Debug("news API non-200", "status", resp.StatusCode)
		return
	}
	body, err := safe.ReadAllLimited(resp.Body, 1024*1024) // 1MB limit
	if err != nil {
		return
	}

	var result struct {
		Data []struct {
			Title       string `json:"title"`
			Source      string `json:"source"`
			URL         string `json:"url"`
			Body        string `json:"body"`
			Categories  string `json:"categories"`
			PublishedOn int64  `json:"published_on"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	bullish := []string{"surge", "rally", "bullish", "breakout", "ath", "pump", "adoption"}
	bearish := []string{"crash", "dump", "bearish", "sell-off", "plunge", "hack", "ban", "fraud"}

	for _, d := range result.Data {
		if seen[d.URL] {
			continue
		}
		seen[d.URL] = true
		*seenOrder = append(*seenOrder, d.URL)
		if time.Since(time.Unix(d.PublishedOn, 0)) > 10*time.Minute {
			continue
		}

		lower := strings.ToLower(d.Title + " " + d.Body)
		bc, brc := 0, 0
		for _, w := range bullish {
			if strings.Contains(lower, w) {
				bc++
			}
		}
		for _, w := range bearish {
			if strings.Contains(lower, w) {
				brc++
			}
		}

		if bc == 0 && brc == 0 {
			continue
		}

		emoji := "📰"
		sentiment := "NEUTRAL"
		if bc > brc {
			emoji = "🟢"
			sentiment = "BULLISH"
		}
		if brc > bc {
			emoji = "🔴"
			sentiment = "BEARISH"
		}

		b.agent.notifyAll(fmt.Sprintf("%s *News*\n\n%s\n\n• Source: %s\n• Sentiment: %s",
			emoji, d.Title, d.Source, sentiment))
	}

	// Evict the oldest half when seen grows large so recent URLs stay deduped deterministically.
	if len(seen) > 1000 {
		half := len(seen) / 2
		for i := 0; i < half && i < len(*seenOrder); i++ {
			delete(seen, (*seenOrder)[i])
		}
		if half < len(*seenOrder) {
			*seenOrder = append((*seenOrder)[:0], (*seenOrder)[half:]...)
		} else {
			*seenOrder = (*seenOrder)[:0]
		}
	}
}

func (b *Brain) StartMarketBriefs(hours []int) {
	safe.GoNamed("brain-market-briefs", func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		sent := make(map[string]bool)
		for {
			select {
			case <-b.stopCh:
				return
			case now := <-ticker.C:
				key := now.Format("2006-01-02-15")
				for _, h := range hours {
					if now.Hour() == h && now.Minute() == 30 && !sent[key] {
						sent[key] = true
						b.sendBrief(h)
					}
				}
			}
		}
	})
}

func (b *Brain) sendBrief(hour int) {
	title := "☀️ *早间市场简报*"
	if hour >= 18 {
		title = "🌙 *晚间市场简报*"
	}

	// Fetch BTC/ETH prices for the brief
	var btcPrice, ethPrice, btcChg, ethChg string
	for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
		resp, err := b.http.Get(fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", sym))
		if err != nil {
			continue
		}
		body, readErr := safe.ReadAllLimited(resp.Body, 64*1024) // 64KB limit
		statusOK := resp.StatusCode == http.StatusOK
		resp.Body.Close()
		if readErr != nil || !statusOK {
			continue
		}
		var t map[string]string
		if err := json.Unmarshal(body, &t); err != nil {
			continue
		}
		if sym == "BTCUSDT" {
			btcPrice = t["lastPrice"]
			btcChg = t["priceChangePercent"]
		}
		if sym == "ETHUSDT" {
			ethPrice = t["lastPrice"]
			ethChg = t["priceChangePercent"]
		}
	}

	brief := fmt.Sprintf("%s\n\n• BTC: $%s (%s%%)\n• ETH: $%s (%s%%)\n\n_%s_",
		title, btcPrice, btcChg, ethPrice, ethChg, time.Now().Format("2006-01-02 15:04"))

	b.agent.notifyAll(brief)
}
