package stockengine

import (
	"math"
	"strconv"
	"strings"
	"time"

	"nofx/market/usstock"
)

// universeMinBars is the daily history BuildUniverseRow needs: 30 returns for
// the 30-bar volatility and return windows.
const universeMinBars = 31

// universeETFs are the underlyings known to be ETFs/ETNs without a fundamentals
// lookup; everything else is "stock" only once fundamentals say so.
var universeETFs = map[string]bool{
	"SPY": true, "QQQ": true, "TQQQ": true, "SQQQ": true, "SOXL": true, "SOXS": true,
	"UVXY": true, "SMH": true, "EWY": true, "KORU": true, "MUU": true,
}

// BuildUniverseRow computes the cross-sectional row of one pair from its daily
// bars (ascending, closed). It returns false with fewer than 31 bars or a
// non-positive close in the windows used. Class falls back to the static ETF
// set, Sector to "" and DaysToEarnings to −1; ApplyFundamentals refines them.
func BuildUniverseRow(symbol, underlying string, bars []usstock.Bar) (UniverseRow, bool) {
	n := len(bars)
	if n < universeMinBars {
		return UniverseRow{}, false
	}
	window := bars[n-universeMinBars:] // 31 bars → 30 returns
	for _, b := range window {
		if b.Close <= 0 {
			return UniverseRow{}, false
		}
	}
	price := bars[n-1].Close
	row := UniverseRow{Symbol: symbol, Underlying: underlying, Price: price, DaysToEarnings: -1}
	if universeETFs[strings.ToUpper(underlying)] {
		row.Class = "ETF"
	}
	row.Ret1dPct = (price/bars[n-2].Close - 1) * 100
	row.Ret10dPct = (price/bars[n-11].Close - 1) * 100
	row.Ret30dPct = (price/bars[n-31].Close - 1) * 100

	rets := make([]float64, 0, 30)
	for i := 1; i < len(window); i++ {
		rets = append(rets, math.Log(window[i].Close/window[i-1].Close))
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	variance := 0.0
	for _, r := range rets {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(rets) - 1)
	row.Vol30Pct = math.Sqrt(variance) * math.Sqrt(252) * 100

	row.RSI14 = rsi(closes(bars), rsiPeriod)

	high := 0.0
	for _, b := range bars[n-30:] {
		if b.High > high {
			high = b.High
		}
	}
	if high > 0 {
		row.DistFrom30dHighPct = math.Min(0, (price/high-1)*100)
	}
	adv := 0.0
	for _, b := range bars[n-20:] {
		adv += b.Close * b.Volume
	}
	row.AvgDollarVol20 = adv / 20
	return row, true
}

// ApplyFundamentals overlays best-effort fundamentals on a row: Class from the
// quote type (ETF / EQUITY), Sector, and DaysToEarnings as the number of ET
// calendar days from now to the next earnings date (−1 when unknown or past).
func ApplyFundamentals(row *UniverseRow, f usstock.Fundamentals, now time.Time) {
	switch strings.ToUpper(f.QuoteType) {
	case "ETF":
		row.Class = "ETF"
	case "EQUITY":
		row.Class = "stock"
	}
	if f.Sector != "" {
		row.Sector = f.Sector
	}
	row.DaysToEarnings = daysUntil(f.NextEarnings, now)
}

// daysUntil is the ET calendar-day distance from now to date (≥ 0), or −1
// when date is zero or before today's ET date.
func daysUntil(date, now time.Time) int {
	if date.IsZero() {
		return -1
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}
	d, n := date.In(loc), now.In(loc)
	a := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	days := int(a.Sub(b).Hours() / 24)
	if days < 0 {
		return -1
	}
	return days
}

// fmtPx renders a price compactly: 4 decimals below 1, otherwise 4 significant
// digits (never dropping integer digits), trailing zeros trimmed.
func fmtPx(v float64) string {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	decimals := 4
	if a := math.Abs(v); a >= 1 {
		decimals = 3 - int(math.Floor(math.Log10(a)))
		if decimals < 0 {
			decimals = 0
		}
	}
	scale := math.Pow(10, float64(decimals))
	return strconv.FormatFloat(math.Round(v*scale)/scale, 'f', -1, 64)
}

// fmtVol renders a volume or dollar amount with three significant digits and a
// K/M/B/T suffix: 950, 12.3K, 1.23M, 4.5B.
func fmtVol(v float64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	if math.Round(v) < 1000 {
		return sign + strconv.FormatFloat(math.Round(v), 'f', 0, 64)
	}
	units := []string{"K", "M", "B", "T"}
	x, u := v/1000, 0
	for {
		decimals := 2
		if x >= 100 {
			decimals = 0
		} else if x >= 10 {
			decimals = 1
		}
		scale := math.Pow(10, float64(decimals))
		r := math.Round(x*scale) / scale
		if r >= 1000 && u < len(units)-1 {
			x, u = x/1000, u+1
			continue
		}
		return sign + strconv.FormatFloat(r, 'f', -1, 64) + units[u]
	}
}
