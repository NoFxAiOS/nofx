package vergex

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"nofx/logger"
	"nofx/telegram/notify"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// StartHeadlessRelay drives a real (headless) Chrome through the vergex.trade
// Cloudflare challenge on a timer and re-fetches the endpoints the strategy
// engine consumes, feeding the in-memory relay cache (relay.go). This makes
// vergex data (AI500 coin list, OI ranking, NetFlow) independent of the data
// page being open in a human's browser.
//
// Disable with VERGEX_HEADLESS=off. Set VERGEX_HEADLESS_SHOW=true to run the
// browser headed (useful if Cloudflare tightens headless detection).
func StartHeadlessRelay(ctx context.Context, interval time.Duration) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("VERGEX_HEADLESS")))
	if mode == "off" || mode == "false" || mode == "0" {
		logger.Info("🧭 Vergex headless relay disabled (VERGEX_HEADLESS=off)")
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	go runRelay(ctx, interval)
}

// headlessEndpoints are the strategy-facing vergex endpoints: the AI500 coin
// list, the OI ranking tab (coin sources), and the fund-flow ranking. Price
// rankings are served from Binance directly and no longer need the relay.
func headlessEndpoints() []string {
	return []string{
		"/trending-category?lang=en&key=ai500&limit=100",
		"/trending-crypto?tab=oi&duration=1h&limit=100",
		"/api/v1/data-intelligence/flow/markets?limit=25&window=1h",
	}
}

const (
	openTimeout  = 60 * time.Second // browser launch-to-page and Cloudflare settle
	fetchTimeout = 30 * time.Second // one in-page fetch
	// relayMaxBackoff caps the retry delay while vergex stays unreachable.
	relayMaxBackoff = 5 * time.Minute
)

type relaySession struct {
	launcher *launcher.Launcher
	browser  *rod.Browser
	page     *rod.Page
}

// close tears down whatever partial state exists. The launcher is killed even
// when the browser never connected — otherwise a failed open() leaks the whole
// Chromium process tree.
func (s *relaySession) close() {
	if s.page != nil {
		_ = s.page.Close()
		s.page = nil
	}
	if s.browser != nil {
		_ = s.browser.Close()
		s.browser = nil
	}
	if s.launcher != nil {
		l := s.launcher
		s.launcher = nil
		l.Kill()
		// Cleanup removes the temp user-data-dir but blocks on the process-exit
		// channel, which may never close on a rare rod error path: never inline.
		go l.Cleanup()
	}
}

// open launches a fresh browser session and navigates to the trending page so
// the Cloudflare challenge resolves inside a real browser context.
func (s *relaySession) open() error {
	s.close()

	headless := strings.ToLower(os.Getenv("VERGEX_HEADLESS_SHOW")) != "true"
	l := launcher.New().
		Headless(headless).
		Set("disable-blink-features", "AutomationControlled")
	s.launcher = l
	controlURL, err := l.Launch()
	if err != nil {
		s.close()
		return fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(controlURL)
	s.browser = browser
	if err := browser.Connect(); err != nil {
		s.close()
		return fmt.Errorf("connect browser: %w", err)
	}

	page, err := browser.Timeout(openTimeout).Page(proto.TargetCreateTarget{URL: "https://vergex.trade/trending"})
	if err != nil {
		s.close()
		return fmt.Errorf("open vergex page: %w", err)
	}
	page = page.CancelTimeout()
	s.page = page

	// Give the Cloudflare challenge time to resolve before the first fetches.
	waitPage := page.Timeout(openTimeout)
	if err := waitPage.WaitStable(8 * time.Second); err != nil {
		logger.Infof("🧭 Vergex headless: page wait interrupted: %v", err)
	}
	waitPage.CancelTimeout()
	return nil
}

// fetchAll fetches every endpoint in the live page context (origin cookies and
// TLS fingerprint are the real browser's) and feeds the relay cache.
func (s *relaySession) fetchAll() error {
	if s.page == nil {
		if err := s.open(); err != nil {
			return err
		}
	}
	for _, endpoint := range headlessEndpoints() {
		p := s.page.Timeout(fetchTimeout)
		res, err := p.Eval(
			`(u) => fetch(u, {headers: {accept: 'application/json'}}).then(r => r.text())`,
			"https://vergex.trade"+endpoint,
		)
		p.CancelTimeout()
		if err != nil {
			return fmt.Errorf("eval %s: %w", endpoint, err)
		}
		body := strings.TrimSpace(res.Value.Str())
		if len(body) == 0 || body[0] != '{' {
			return fmt.Errorf("%s: non-JSON response (challenge page?)", endpoint)
		}
		FeedRelay(endpoint, []byte(body))
	}
	return nil
}

// relayRetryDelay is the wait before the next attempt: the normal interval
// while healthy, then interval * 2^(n-1) after n consecutive failures, capped
// at relayMaxBackoff (interval=1m: 1,2,4,5,5...).
func relayRetryDelay(failures int, interval time.Duration) time.Duration {
	if failures <= 0 {
		return interval
	}
	d := interval
	for i := 1; i < failures; i++ {
		d *= 2
		if d >= relayMaxBackoff {
			return relayMaxBackoff
		}
	}
	if d > relayMaxBackoff {
		return relayMaxBackoff
	}
	return d
}

// relayHealth tracks consecutive failures (for backoff) and decides when an
// outage is long enough to alert: exactly when cached data stops being served
// (relayTTL without a success). One alert per outage.
type relayHealth struct {
	lastSuccess time.Time // initialised to relay start time
	everOK      bool
	failures    int
	alerted     bool
	lastErr     error
	lastOutage  time.Duration // length of the outage just recovered from
}

func newRelayHealth(start time.Time) *relayHealth {
	return &relayHealth{lastSuccess: start}
}

// record registers one attempt result and reports whether to send an outage
// alert or a recovery notice.
func (h *relayHealth) record(ok bool, err error, now time.Time) (alert, recovered bool) {
	if ok {
		recovered = h.alerted
		h.lastOutage = now.Sub(h.lastSuccess)
		h.alerted = false
		h.failures = 0
		h.lastErr = nil
		h.lastSuccess = now
		h.everOK = true
		return false, recovered
	}
	h.failures++
	h.lastErr = err
	if !h.alerted && now.Sub(h.lastSuccess) >= relayTTL {
		h.alerted = true
		return true, false
	}
	return false, false
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (h *relayHealth) alertMessage() string {
	last := "启动后从未成功"
	if h.everOK {
		last = h.lastSuccess.Format("01-02 15:04:05")
	}
	errText := "-"
	if h.lastErr != nil {
		errText = truncateRunes(h.lastErr.Error(), 200)
	}
	return fmt.Sprintf("<b>⚠️ Vergex 数据超过 %d 分钟未拉取成功</b>\n最近成功: %s\n连续失败: %d 次\n最近错误: %s\n缓存数据已过期，AI500 / OI 排行 / 资金流选币源暂不可用",
		int(relayTTL/time.Minute), last, h.failures, notify.Escape(errText))
}

func runRelay(ctx context.Context, interval time.Duration) {
	session := &relaySession{}
	defer session.close()

	health := newRelayHealth(time.Now())
	attempt := func() bool {
		err := session.fetchAll()
		now := time.Now()
		if err != nil {
			session.close()
		}
		alert, recovered := health.record(err == nil, err, now)
		if err != nil {
			logger.Infof("🧭 Vergex headless relay failed: %v — restarting browser session, retry in %s",
				err, relayRetryDelay(health.failures, interval))
		}
		if alert {
			logger.Warnf("🧭 Vergex headless relay: no success for %s, cached data expired", relayTTL)
			notify.Notify("ALERT", "Vergex 数据源", health.alertMessage())
		}
		if recovered {
			down := health.lastOutage.Round(time.Minute)
			logger.Infof("🧭 Vergex headless relay recovered after %s", down)
			notify.Notify("INFO", "Vergex 数据源", fmt.Sprintf("<b>✅ Vergex 数据已恢复</b>\n中断时长: %s", down))
		}
		return err == nil
	}

	// First round with a few spaced retries (browser cold start can be slow).
	for i := 0; i < 3; i++ {
		if attempt() {
			logger.Infof("🧭 Vergex headless relay active: %d endpoints refreshed every %s",
				len(headlessEndpoints()), interval)
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}

	timer := time.NewTimer(relayRetryDelay(health.failures, interval))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			attempt()
			timer.Reset(relayRetryDelay(health.failures, interval))
		}
	}
}
