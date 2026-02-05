package gate

import (
	"testing"
	"time"

	"github.com/gateio/gateapi-go/v6"
	"github.com/stretchr/testify/require"
)

func findByAction(trades []GateTrade, action string) *GateTrade {
	for i := range trades {
		if trades[i].OrderAction == action {
			return &trades[i]
		}
	}
	return nil
}

func TestExpandGateTrade_OpenLong(t *testing.T) {
	execTime := time.Unix(1700000000, 0).UTC()
	trade := gateapi.MyFuturesTrade{
		Id:        1,
		Contract:  "BTC_USDT",
		OrderId:   "o1",
		Size:      10,
		CloseSize: 0,
	}

	parts := expandGateTrade(trade, 30000, 0.1, 0.001, execTime)
	require.Len(t, parts, 1)

	p := parts[0]
	require.Equal(t, "open_long", p.OrderAction)
	require.Equal(t, "BUY", p.Side)
	require.Equal(t, "1", p.TradeID)
	require.InDelta(t, 0.01, p.FillQty, 1e-12)  // 10 contracts * 0.001
	require.InDelta(t, 0.1, p.Fee, 1e-12)
}

func TestExpandGateTrade_SplitCloseAndOpen(t *testing.T) {
	execTime := time.Unix(1700000000, 0).UTC()
	trade := gateapi.MyFuturesTrade{
		Id:        2,
		Contract:  "BTC_USDT",
		OrderId:   "o2",
		Size:      10,
		CloseSize: 4,
	}

	parts := expandGateTrade(trade, 30000, 0.2, 0.001, execTime)
	require.Len(t, parts, 2)

	closePart := findByAction(parts, "close_short")
	openPart := findByAction(parts, "open_long")
	require.NotNil(t, closePart)
	require.NotNil(t, openPart)

	require.Equal(t, "BUY", closePart.Side)
	require.Equal(t, "2:close", closePart.TradeID)
	require.InDelta(t, 0.004, closePart.FillQty, 1e-12) // 4 * 0.001
	require.InDelta(t, 0.08, closePart.Fee, 1e-12)      // 0.2 * 4/10

	require.Equal(t, "BUY", openPart.Side)
	require.Equal(t, "2:open", openPart.TradeID)
	require.InDelta(t, 0.006, openPart.FillQty, 1e-12) // 6 * 0.001
	require.InDelta(t, 0.12, openPart.Fee, 1e-12)      // 0.2 * 6/10
}

func TestExpandGateTrade_CloseLongOnly(t *testing.T) {
	execTime := time.Unix(1700000000, 0).UTC()
	trade := gateapi.MyFuturesTrade{
		Id:        3,
		Contract:  "ETH_USDT",
		OrderId:   "o3",
		Size:      -5,
		CloseSize: -8,
	}

	parts := expandGateTrade(trade, 2000, 0.05, 0.01, execTime)
	require.Len(t, parts, 1)

	p := parts[0]
	require.Equal(t, "close_long", p.OrderAction)
	require.Equal(t, "SELL", p.Side)
	require.Equal(t, "3", p.TradeID)
	require.InDelta(t, 0.05, p.FillQty, 1e-12) // 5 * 0.01
	require.InDelta(t, 0.05, p.Fee, 1e-12)
}
