package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/market"
)

func TestAttachQuantContextToMarketData(t *testing.T) {
	m := map[string]*market.Data{"BTCUSDT": {Symbol: "BTCUSDT"}}
	q := map[string]*kernel.QuantData{"BTCUSDT": {
		Symbol: "BTCUSDT",
		Netflow: &kernel.NetflowData{
			Institution: &kernel.FlowTypeData{Future: map[string]float64{"1h": 1000}},
			Personal:    &kernel.FlowTypeData{Future: map[string]float64{"1h": 500}},
		},
		OI: map[string]*kernel.OIData{"binance": {Delta: map[string]*kernel.OIDeltaData{"1h": {OIDeltaPercent: 9}}}},
	}}
	attachQuantContextToMarketData(m, q)
	if m["BTCUSDT"].QuantContext == nil || m["BTCUSDT"].QuantContext.CrowdingRisk != "high" {
		t.Fatalf("expected high crowding quant context, got %+v", m["BTCUSDT"].QuantContext)
	}
}
