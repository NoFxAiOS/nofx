package usstock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nofx/market/bnweight"
)

// Tests replace these variables before starting requests.
var (
	httpClient           = &http.Client{Timeout: 20 * time.Second, Transport: bnweight.Wrap(nil, bnweight.Normal)}
	spotBaseURL          = "https://api.binance.com"
	futuresBaseURL       = "https://fapi.binance.com"
	yahooBaseURL         = "https://query1.finance.yahoo.com"
	yahooFallbackBaseURL = "https://query2.finance.yahoo.com"
	nowFunc              = time.Now
)

type httpStatusError struct {
	status   int
	endpoint string
}

func (e *httpStatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.endpoint, e.status) }

func fetchJSON(ctx context.Context, base, path string, query url.Values, yahoo bool, dst any) error {
	endpoint := strings.TrimRight(base, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if yahoo {
		req.Header.Set("User-Agent", "Mozilla/5.0")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{resp.StatusCode, endpoint}
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(dst)
}
