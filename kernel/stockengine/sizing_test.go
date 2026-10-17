package stockengine

import "testing"

func TestSizeOrderBounds(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Context)
		qty     float64
		limited string
	}{
		{"risk", func(c *Context) {}, 10, "risk"},
		{"position", func(c *Context) { c.Config.MaxPositionPct = 5 }, 5, "position_cap"},
		{"exposure", func(c *Context) { c.Account.Exposure = 7500 }, 5, "exposure_cap"},
		{"available", func(c *Context) { c.Account.Available = 500 }, 4.975, "available"},
		{"position net of holding", func(c *Context) { holding(c); c.Config.MaxPositionPct = 10 }, 5, "position_cap"},
		{"position uses snapshot mark", func(c *Context) { holding(c); c.Positions[0].Price = 90; c.Config.MaxPositionPct = 10 }, 5, "position_cap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testContext()
			tt.setup(c)
			d := buy("AAPLBUSDT")
			d.StopLoss = 90
			s, err := SizeOrder(c, d)
			if err != nil {
				t.Fatal(err)
			}
			near(t, s.Quantity, tt.qty)
			near(t, s.Notional, tt.qty*100)
			near(t, s.RiskUSDT, tt.qty*10)
			if s.LimitedBy != tt.limited {
				t.Fatalf("got %s want %s", s.LimitedBy, tt.limited)
			}
		})
	}
}

func TestSizeOrderErrors(t *testing.T) {
	tests := []struct {
		name, code string
		setup      func(*Context, *Decision)
	}{
		{"tiny", "MIN_NOTIONAL", func(c *Context, d *Decision) { c.Account.Available = 5 }},
		{"empty cash", "NO_CAPACITY", func(c *Context, d *Decision) { c.Account.Available = 0 }},
		{"overexposed", "NO_CAPACITY", func(c *Context, d *Decision) { c.Account.Exposure = 9000 }},
		{"position full", "NO_CAPACITY", func(c *Context, d *Decision) { holding(c); c.Config.MaxPositionPct = 4 }},
		{"invalid risk", "STOP_INVALID", func(c *Context, d *Decision) { d.StopLoss = 100 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testContext()
			d := buy("AAPLBUSDT")
			tt.setup(c, &d)
			s, err := SizeOrder(c, d)
			if err == nil || err.Error() != tt.code {
				t.Fatalf("size=%+v error=%v want %s", s, err, tt.code)
			}
			if tt.code == "MIN_NOTIONAL" && s.LimitedBy != "min_notional" {
				t.Fatal(s)
			}
		})
	}
}

func TestSizeOrderUsesLimitEntry(t *testing.T) {
	c := testContext()
	d := buy("AAPLBUSDT")
	d.EntryType = EntryLimit
	d.LimitPrice = 98
	d.StopLoss = 88
	s, err := SizeOrder(c, d)
	if err != nil {
		t.Fatal(err)
	}
	near(t, s.Quantity, 10)
	near(t, s.Notional, 980)
	near(t, s.RiskUSDT, 100)
}

func TestSizeOrderRegimeScalesBounds(t *testing.T) {
	half := RegimeScale{Enabled: true, Scale: 0.5}
	tests := []struct {
		name    string
		setup   func(*Context)
		qty     float64
		limited string
	}{
		// equity 10000, entry 100, stop 90 (risk 10/share), default risk 1%.
		{"risk budget halves", func(c *Context) {}, 5, "risk"},
		{"position cap halves", func(c *Context) { c.Config.RiskPerTradePct = 5; c.Config.MaxPositionPct = 10 }, 5, "position_cap"},
		{"position cap nets the holding", func(c *Context) {
			holding(c)
			c.Config.RiskPerTradePct = 5
			c.Config.MaxPositionPct = 10 // 10000×10%×0.5 = 500 − 5×100 held = 0
		}, 0, ""},
		{"exposure cap halves", func(c *Context) { c.Config.RiskPerTradePct = 5; c.Account.Exposure = 3500 }, 5, "exposure_cap"}, // 4000 − 3500
		{"available is not scaled", func(c *Context) { c.Account.Available = 500 }, 4.975, "available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testContext()
			c.Regime = half
			tt.setup(c)
			d := buy("AAPLBUSDT")
			d.StopLoss = 90
			s, err := SizeOrder(c, d)
			if tt.qty == 0 {
				if err == nil || err.Error() != "NO_CAPACITY" {
					t.Fatalf("size=%+v err=%v want NO_CAPACITY", s, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			near(t, s.Quantity, tt.qty)
			if s.LimitedBy != tt.limited {
				t.Fatalf("limited by %s want %s", s.LimitedBy, tt.limited)
			}
		})
	}
}

func TestSizeOrderRegimeDisabledIsIdentical(t *testing.T) {
	d := buy("AAPLBUSDT")
	d.StopLoss = 90
	for _, regime := range []RegimeScale{
		{},                           // zero value
		{Enabled: false, Scale: 0.3}, // a stale scale must be ignored when disabled
		{Enabled: true, Scale: 1},
	} {
		for _, setup := range []func(*Context){
			func(c *Context) {},
			func(c *Context) { c.Config.MaxPositionPct = 5 },
			func(c *Context) { c.Account.Exposure = 7500 },
			func(c *Context) { holding(c); c.Config.MaxPositionPct = 10 },
		} {
			base, with := testContext(), testContext()
			setup(base)
			setup(with)
			with.Regime = regime
			want, wantErr := SizeOrder(base, d)
			got, gotErr := SizeOrder(with, d)
			if got != want || (gotErr == nil) != (wantErr == nil) {
				t.Fatalf("regime %+v changed sizing: got %+v/%v want %+v/%v", regime, got, gotErr, want, wantErr)
			}
		}
	}
}
