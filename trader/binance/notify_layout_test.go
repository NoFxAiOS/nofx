package binance

import (
	"strings"
	"testing"
)

// commaFormat: thousands separators in the integer part only.
func TestCommaFormat(t *testing.T) {
	cases := map[float64]string{
		1425:     "1,425",
		1234567:  "1,234,567",
		0.003289: "0.003289",
		0.0023:   "0.0023",
		4.687825: "4.687825",
		-0.1:     "-0.1",
		2526.7:   "2,526.7",
		0:        "0",
	}
	for in, want := range cases {
		if got := commaFormat(in); got != want {
			t.Errorf("commaFormat(%v) = %q, want %q", in, got, want)
		}
	}
}

// Fill message layout: blank-line groups so the PnL block stands out on a
// phone screen instead of a cramped wall of text.
func TestNotifyFillLayoutStructure(t *testing.T) {
	msg := "<b>🔴 平多 1000BONKUSDT</b>\n\n" +
		"数量  <code>" + commaFormat(1425) + "</code>\n" +
		"价格  <code>" + commaFormat(0.003289) + "</code>\n" +
		"价值  <code>" + commaFormat(1425*0.003289) + " USDT</code>\n" +
		"手续费  <code>" + commaFormat(0.0023) + " USDT</code>" +
		"\n\n━━━━━━━━━━\n\n已实现盈亏（扣手续费）\n❌ 亏损 <b>-0.10 USDT</b>\n<i>毛利 -0.08 · 手续费 -0.02 · 未含资金费</i>\n\n<i>成交 09-05 03:21:48 (UTC)</i>"

	for _, want := range []string{
		"<b>🔴 平多 1000BONKUSDT</b>\n\n数量",
		"1,425",
		"\n\n━━━━━━━━━━\n\n已实现盈亏（扣手续费）\n❌ 亏损 <b>-0.10 USDT</b>",
		"\n\n<i>成交",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("layout missing %q:\n%s", want, msg)
		}
	}
}
