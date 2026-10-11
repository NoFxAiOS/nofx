package bnweight

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Bucket says which Binance rate-limit pool a request is charged to.
type Bucket int

const (
	// BucketIP1M is the request-weight pool (2400/min per IP). Its authoritative
	// usage is the X-MBX-USED-WEIGHT-1M response header.
	BucketIP1M Bucket = iota
	// BucketFundingRate is /fapi/v1/fundingRate + fundingInfo: a separate
	// 500 requests / 5 min / IP limit that does NOT consume request weight.
	BucketFundingRate
	// BucketData is /futures/data/*: a separate 1000 requests / 5 min / IP limit.
	BucketData
)

func (b Bucket) String() string {
	switch b {
	case BucketFundingRate:
		return "fundingRate"
	case BucketData:
		return "data"
	default:
		return "weight"
	}
}

// Classify returns the normalized endpoint (path only, no query), the request
// weight charged to the 1M pool, and the pool the request belongs to. For
// BucketFundingRate / BucketData the weight is 0 (they are counted as
// requests, not weight). Unknown endpoints cost 1 (the documented default for
// most simple GETs) and are still reported under their own path so a missing
// table entry is visible in the summary.
func Classify(method, path string, q url.Values) (endpoint string, weight int, bucket Bucket) {
	endpoint = normalizePath(path)
	method = strings.ToUpper(method)

	if strings.HasPrefix(endpoint, "/futures/data/") {
		return endpoint, 0, BucketData
	}
	switch endpoint {
	case "/fapi/v1/fundingRate", "/fapi/v1/fundingInfo":
		return endpoint, 0, BucketFundingRate
	}

	hasSymbol := q.Get("symbol") != ""
	limit := atoiDefault(q.Get("limit"), -1)

	switch endpoint {
	case "/fapi/v1/klines", "/fapi/v1/continuousKlines", "/fapi/v1/indexPriceKlines", "/fapi/v1/markPriceKlines":
		return endpoint, klineWeight(limit), BucketIP1M
	case "/fapi/v1/depth":
		return endpoint, depthWeight(limit), BucketIP1M
	case "/fapi/v1/userTrades":
		return endpoint, 5, BucketIP1M
	case "/fapi/v1/income":
		return endpoint, 30, BucketIP1M
	case "/fapi/v1/positionRisk", "/fapi/v2/positionRisk", "/fapi/v3/positionRisk",
		"/fapi/v1/account", "/fapi/v2/account", "/fapi/v3/account",
		"/fapi/v2/balance", "/fapi/v3/balance":
		return endpoint, 5, BucketIP1M
	case "/fapi/v1/openOrders":
		if hasSymbol {
			return endpoint, 1, BucketIP1M
		}
		return endpoint, 40, BucketIP1M
	case "/fapi/v1/allOrders":
		return endpoint, 5, BucketIP1M
	case "/fapi/v1/ticker/24hr":
		if hasSymbol {
			return endpoint, 1, BucketIP1M
		}
		return endpoint, 40, BucketIP1M
	case "/fapi/v1/ticker/price", "/fapi/v2/ticker/price", "/fapi/v1/ticker/bookTicker":
		if hasSymbol {
			return endpoint, 1, BucketIP1M
		}
		return endpoint, 2, BucketIP1M
	case "/fapi/v1/premiumIndex":
		if hasSymbol {
			return endpoint, 1, BucketIP1M
		}
		return endpoint, 10, BucketIP1M
	case "/fapi/v1/exchangeInfo":
		return endpoint, 1, BucketIP1M
	case "/fapi/v1/commissionRate":
		return endpoint, 20, BucketIP1M
	case "/fapi/v1/batchOrders":
		if method == http.MethodPost {
			return endpoint, 5, BucketIP1M
		}
		return endpoint, 1, BucketIP1M
	case "/fapi/v1/order", "/fapi/v1/algoOrder":
		// A new order costs 0 against the IP weight pool (it has its own order
		// rate limit); query / cancel cost 1.
		if method == http.MethodPost {
			return endpoint, 0, BucketIP1M
		}
		return endpoint, 1, BucketIP1M
	case "/fapi/v1/allOpenOrders":
		return endpoint, 1, BucketIP1M
	case "/fapi/v1/openAlgoOrders":
		if hasSymbol {
			return endpoint, 1, BucketIP1M
		}
		return endpoint, 40, BucketIP1M
	case "/fapi/v1/allAlgoOrders":
		return endpoint, 5, BucketIP1M
	}
	return endpoint, 1, BucketIP1M
}

// klineWeight follows Binance's documented tiers: [1,100) 1, [100,500) 2,
// [500,1000] 5, >1000 10. A missing limit defaults to 500 server-side.
func klineWeight(limit int) int {
	switch {
	case limit < 0:
		return 5 // default limit=500
	case limit < 100:
		return 1
	case limit < 500:
		return 2
	case limit <= 1000:
		return 5
	default:
		return 10
	}
}

// depthWeight: limit 5/10/20/50 -> 2, 100 -> 5, 500 -> 10, 1000 -> 20 (default 500).
func depthWeight(limit int) int {
	switch {
	case limit < 0:
		return 10
	case limit <= 50:
		return 2
	case limit <= 100:
		return 5
	case limit <= 500:
		return 10
	default:
		return 20
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}
