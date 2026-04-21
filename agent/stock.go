package agent

import (
	"nofx/safe"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// stockHTTPClient is a shared HTTP client for stock API requests.
// Reused across calls for connection pooling.
var stockHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}

// StockQuote holds real-time stock data.
type StockQuote struct {
	Name      string
	Code      string
	Market    string  // "A股", "港股", "美股"
	Currency  string  // "CNY", "HKD", "USD"
	Open      float64
	PrevClose float64
	Price     float64
	High      float64
	Low       float64
	Volume    float64
	Turnover  float64
	Date      string
	Time      string
	Change    float64
	ChangePct float64
	// 盘前盘后 (美股)
	ExtPrice     float64 // 盘前/盘后价格
	ExtChangePct float64 // 盘前/盘后涨跌幅%
	ExtChange    float64 // 盘前/盘后涨跌额
	ExtTime      string  // 盘前/盘后时间
	IsExtHours   bool    // 是否在盘前盘后时段
}

// knownStocks maps Chinese names to stock codes.
var knownStocks = map[string]string{
	// A股
	"拓维信息": "sz002261", "比亚迪": "sz002594", "宁德时代": "sz300750",
	"贵州茅台": "sh600519", "中国平安": "sh601318", "招商银行": "sh600036",
	"中芯国际": "sh688981", "工商银行": "sh601398", "建设银行": "sh601939",
	"中国银行": "sh601988", "农业银行": "sh601288", "中信证券": "sh600030",
	"海康威视": "sz002415", "立讯精密": "sz002475", "东方财富": "sz300059",
	"隆基绿能": "sh601012", "长城汽车": "sh601633", "科大讯飞": "sz002230",
	"三六零": "sh601360", "中兴通讯": "sz000063",
	// 港股
	"腾讯": "hk00700", "阿里巴巴": "hk09988", "美团": "hk03690",
	"小米": "hk01810", "京东": "hk09618", "网易": "hk09999",
	"百度": "hk09888", "快手": "hk01024", "哔哩哔哩": "hk09626",
	"理想汽车": "hk02015", "蔚来": "hk09866", "小鹏汽车": "hk09868",
	// 华为 is not publicly listed — removed incorrect Tencent fallback
	// 美股
	"苹果": "gb_aapl", "特斯拉": "gb_tsla", "英伟达": "gb_nvda",
	"微软": "gb_msft", "谷歌": "gb_googl", "亚马逊": "gb_amzn",
	"meta": "gb_meta", "奈飞": "gb_nflx", "台积电": "gb_tsm",
	"拼多多": "gb_pdd", "蔚来汽车": "gb_nio",
}

// US stock ticker mapping
var usTickerMap = map[string]string{
	"AAPL": "gb_aapl", "TSLA": "gb_tsla", "NVDA": "gb_nvda", "MSFT": "gb_msft",
	"GOOGL": "gb_googl", "AMZN": "gb_amzn", "META": "gb_meta", "NFLX": "gb_nflx",
	"TSM": "gb_tsm", "PDD": "gb_pdd", "NIO": "gb_nio", "BABA": "gb_baba",
	"JD": "gb_jd", "BIDU": "gb_bidu", "AMD": "gb_amd", "INTC": "gb_intc",
	"COIN": "gb_coin", "MARA": "gb_mara", "RIOT": "gb_riot",
}

func resolveStockCode(text string) (string, string) {
	// Known Chinese names
	for name, code := range knownStocks {
		if strings.Contains(text, name) {
			return code, name
		}
	}

	// US ticker symbols (uppercase)
	upper := strings.ToUpper(text)
	for ticker, code := range usTickerMap {
		if strings.Contains(upper, ticker) {
			return code, ticker
		}
	}

	// 6-digit A-share code
	for _, w := range strings.Fields(text) {
		w = strings.TrimSpace(w)
		if len(w) == 6 {
			if _, err := strconv.Atoi(w); err == nil {
				prefix := "sz"
				if w[0] == '6' || w[0] == '9' { prefix = "sh" }
				return prefix + w, w
			}
		}
		// 5-digit HK code
		if len(w) == 5 {
			if _, err := strconv.Atoi(w); err == nil {
				return "hk" + w, w
			}
		}
	}

	return "", ""
}

// SearchResult represents a stock search result from Sina suggest API.
type SearchResult struct {
	Name   string // Display name
	Code   string // Sina-style code (e.g. sz300750, hk00700, gb_tsla)
	Ticker string // Raw ticker (e.g. 300750, 00700, tsla)
	Type   string // Market type code: 11=A股, 31=港股, 41=美股
	Market string // "A股", "港股", "美股"
}

// searchStock queries Sina's suggest API for dynamic stock search.
// Returns matching stocks across A-share, HK, and US markets.
func searchStock(keyword string) ([]SearchResult, error) {
	// type=11 (A股), 31 (港股), 41 (美股)
	u := fmt.Sprintf("https://suggest3.sinajs.cn/suggest/type=11,31,41&key=%s&name=suggestdata",
		url.QueryEscape(keyword))

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := stockHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stock search API returned status %d", resp.StatusCode)
	}

	reader := transform.NewReader(io.LimitReader(resp.Body, 256*1024), simplifiedchinese.GBK.NewDecoder())
	body, err := safe.ReadAllLimited(reader)
	if err != nil {
		return nil, err
	}

	line := string(body)
	// Parse: var suggestdata="item1;item2;..."
	start := strings.Index(line, "\"")
	end := strings.LastIndex(line, "\"")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("invalid suggest response")
	}
	data := line[start+1 : end]
	if data == "" {
		return nil, nil // no results
	}

	var results []SearchResult
	items := strings.Split(data, ";")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		fields := strings.Split(item, ",")
		if len(fields) < 5 {
			continue
		}
		// fields: [0]=name, [1]=type, [2]=ticker, [3]=sinaCode, [4]=displayName
		typeCode := fields[1]
		ticker := fields[2]
		sinaCode := fields[3]
		displayName := fields[4]
		if displayName == "" {
			displayName = fields[0]
		}

		var mkt, code string
		switch typeCode {
		case "11": // A股
			mkt = "A股"
			code = sinaCode // already like sz300750, sh600519
			if code == "" {
				// Build from ticker
				prefix := "sz"
				if len(ticker) == 6 && (ticker[0] == '6' || ticker[0] == '9') {
					prefix = "sh"
				}
				code = prefix + ticker
			}
		case "31": // 港股
			mkt = "港股"
			code = "hk" + ticker
		case "41": // 美股
			mkt = "美股"
			code = "gb_" + ticker
		default:
			continue // skip funds (201), indices, etc.
		}

		results = append(results, SearchResult{
			Name:   displayName,
			Code:   code,
			Ticker: ticker,
			Type:   typeCode,
			Market: mkt,
		})
	}

	return results, nil
}

// resolveStockCodeDynamic tries local map first, then falls back to Sina search API.
func resolveStockCodeDynamic(text string) (string, string) {
	// First try the static map
	code, name := resolveStockCode(text)
	if code != "" {
		return code, name
	}

	// Fall back to Sina search API
	// Extract a meaningful search keyword from the text
	keyword := extractStockKeyword(text)
	if keyword == "" {
		return "", ""
	}

	results, err := searchStock(keyword)
	if err != nil || len(results) == 0 {
		return "", ""
	}

	// Return the first (best) result
	return results[0].Code, results[0].Name
}

// extractStockKeyword extracts a likely stock name/ticker from user text.
func extractStockKeyword(text string) string {
	// Remove common prefixes/suffixes that aren't stock names
	text = strings.TrimSpace(text)

	// If the text itself is short enough, use it directly
	// (e.g. "中远海控" or "AAPL")
	if len([]rune(text)) <= 10 {
		return text
	}

	// Try to extract quoted terms first: 「xxx」 or "xxx"
	quotePairs := [][2]string{
		{"「", "」"},
		{"\u201c", "\u201d"},
		{"\u2018", "\u2019"},
		{"\"", "\""},
	}
	for _, pair := range quotePairs {
		if s := strings.Index(text, pair[0]); s >= 0 {
			if e := strings.Index(text[s+len(pair[0]):], pair[1]); e >= 0 {
				return text[s+len(pair[0]) : s+len(pair[0])+e]
			}
		}
	}

	// Look for patterns like "查 XXX", "搜索 XXX", "查一下 XXX"
	for _, prefix := range []string{"查一下", "搜索", "查询", "看看", "搜一下", "查", "看", "search ", "find "} {
		if idx := strings.Index(text, prefix); idx >= 0 {
			rest := strings.TrimSpace(text[idx+len(prefix):])
			// Take the first "word" (either Chinese characters or English word)
			words := strings.Fields(rest)
			if len(words) > 0 {
				return words[0]
			}
		}
	}

	// Last resort: use first few words
	words := strings.Fields(text)
	if len(words) > 0 {
		return words[0]
	}

	return ""
}

func fetchStockQuote(code string) (*StockQuote, error) {
	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", code)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := stockHTTPClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stock quote API returned status %d", resp.StatusCode)
	}

	reader := transform.NewReader(io.LimitReader(resp.Body, 256*1024), simplifiedchinese.GBK.NewDecoder())
	body, err := safe.ReadAllLimited(reader)
	if err != nil { return nil, err }

	line := string(body)
	start := strings.Index(line, "\"")
	end := strings.LastIndex(line, "\"")
	if start == -1 || end <= start { return nil, fmt.Errorf("invalid response") }

	data := line[start+1 : end]
	if data == "" { return nil, fmt.Errorf("empty data for %s", code) }

	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") {
		return parseAShare(code, data)
	} else if strings.HasPrefix(code, "hk") {
		return parseHKShare(code, data)
	} else if strings.HasPrefix(code, "gb_") {
		return parseUSShare(code, data)
	}

	return nil, fmt.Errorf("unsupported market: %s", code)
}

func parseAShare(code, data string) (*StockQuote, error) {
	f := strings.Split(data, ",")
	if len(f) < 32 { return nil, fmt.Errorf("too few fields") }

	q := &StockQuote{Name: f[0], Code: code, Market: "A股", Currency: "CNY"}
	q.Open, _ = strconv.ParseFloat(f[1], 64)
	q.PrevClose, _ = strconv.ParseFloat(f[2], 64)
	q.Price, _ = strconv.ParseFloat(f[3], 64)
	q.High, _ = strconv.ParseFloat(f[4], 64)
	q.Low, _ = strconv.ParseFloat(f[5], 64)
	q.Volume, _ = strconv.ParseFloat(f[8], 64)
	q.Turnover, _ = strconv.ParseFloat(f[9], 64)
	q.Date = f[30]; q.Time = f[31]
	if q.PrevClose > 0 { q.Change = q.Price - q.PrevClose; q.ChangePct = (q.Change / q.PrevClose) * 100 }
	return q, nil
}

func parseHKShare(code, data string) (*StockQuote, error) {
	f := strings.Split(data, ",")
	if len(f) < 18 { return nil, fmt.Errorf("too few fields") }

	q := &StockQuote{Name: f[1], Code: code, Market: "港股", Currency: "HKD"}
	q.PrevClose, _ = strconv.ParseFloat(f[3], 64)
	q.Open, _ = strconv.ParseFloat(f[2], 64)
	q.High, _ = strconv.ParseFloat(f[4], 64)
	q.Low, _ = strconv.ParseFloat(f[5], 64)
	q.Price, _ = strconv.ParseFloat(f[6], 64)
	q.Change, _ = strconv.ParseFloat(f[7], 64)
	q.ChangePct, _ = strconv.ParseFloat(f[8], 64)
	q.Turnover, _ = strconv.ParseFloat(f[10], 64)
	q.Volume, _ = strconv.ParseFloat(f[11], 64)
	if len(f) > 17 { q.Date = f[17]; q.Time = f[17] }
	return q, nil
}

func parseUSShare(code, data string) (*StockQuote, error) {
	f := strings.Split(data, ",")
	if len(f) < 30 { return nil, fmt.Errorf("too few fields") }

	q := &StockQuote{Name: f[0], Code: code, Market: "美股", Currency: "USD"}
	q.Price, _ = strconv.ParseFloat(f[1], 64)
	q.ChangePct, _ = strconv.ParseFloat(f[2], 64)
	q.Change, _ = strconv.ParseFloat(f[4], 64)
	q.Open, _ = strconv.ParseFloat(f[5], 64)
	q.High, _ = strconv.ParseFloat(f[6], 64)
	q.Low, _ = strconv.ParseFloat(f[7], 64)
	// 52wk high/low
	high52, _ := strconv.ParseFloat(f[8], 64)
	low52, _ := strconv.ParseFloat(f[9], 64)
	q.Volume, _ = strconv.ParseFloat(f[10], 64)
	q.Turnover, _ = strconv.ParseFloat(f[11], 64)
	if len(f) > 25 { q.Date = f[25]; q.Time = f[26] }
	q.PrevClose = q.Price - q.Change
	_ = high52; _ = low52

	// 盘前盘后数据 (字段21=价格, 22=涨跌幅%, 23=涨跌额, 24=时间)
	if len(f) > 24 {
		extPrice, _ := strconv.ParseFloat(f[21], 64)
		extPct, _ := strconv.ParseFloat(f[22], 64)
		extChg, _ := strconv.ParseFloat(f[23], 64)
		if extPrice > 0 {
			q.ExtPrice = extPrice
			q.ExtChangePct = extPct
			q.ExtChange = extChg
			q.ExtTime = strings.TrimSpace(f[24])
			q.IsExtHours = true
		}
	}

	return q, nil
}

func formatStockQuote(q *StockQuote) string {
	emoji := "🟢"
	if q.ChangePct < 0 { emoji = "🔴" }

	sym := "¥"
	if q.Currency == "USD" { sym = "$" }
	if q.Currency == "HKD" { sym = "HK$" }

	volStr := fmt.Sprintf("%.0f", q.Volume)
	if q.Volume > 1000000 { volStr = fmt.Sprintf("%.1f万", q.Volume/10000) }
	if q.Volume > 100000000 { volStr = fmt.Sprintf("%.2f亿", q.Volume/100000000) }

	turnStr := fmt.Sprintf("%.0f", q.Turnover)
	if q.Turnover > 100000000 { turnStr = fmt.Sprintf("%.2f亿", q.Turnover/100000000) }

	result := fmt.Sprintf(`%s *%s* (%s · %s)
💰 现价: %s%.2f (%+.2f%%)
📊 开盘: %s%.2f | 昨收: %s%.2f
📈 最高: %s%.2f | 最低: %s%.2f
📦 成交: %s | 额: %s
🕐 %s`,
		emoji, q.Name, q.Code, q.Market,
		sym, q.Price, q.ChangePct,
		sym, q.Open, sym, q.PrevClose,
		sym, q.High, sym, q.Low,
		volStr, turnStr,
		q.Date)

	// 盘前盘后数据
	if q.IsExtHours && q.ExtPrice > 0 {
		extEmoji := "🟢"
		if q.ExtChangePct < 0 { extEmoji = "🔴" }
		extLabel := "🌙 盘后"
		if strings.Contains(strings.ToLower(q.ExtTime), "am") {
			extLabel = "🌅 盘前"
		}
		result += fmt.Sprintf("\n%s %s: %s%.2f (%+.2f%%) %s",
			extLabel, extEmoji, sym, q.ExtPrice, q.ExtChangePct, q.ExtTime)
	}

	return result
}
