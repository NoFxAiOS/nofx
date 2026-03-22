package agent

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

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
	"华为": "hk00700", // fallback to tencent for now
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

func fetchStockQuote(code string) (*StockQuote, error) {
	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", code)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
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

	return fmt.Sprintf(`%s *%s* (%s · %s)
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
}
