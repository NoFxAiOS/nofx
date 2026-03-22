package interaction

import (
	"fmt"
	"strings"
	"time"
)

// FormatTradeAlert formats a trade event for Telegram notification.
func FormatTradeAlert(action, symbol, exchange string, price, quantity float64) string {
	emoji := "🟢"
	if action == "sell" || action == "short" || action == "close" {
		emoji = "🔴"
	}
	return fmt.Sprintf(`%s *Trade Executed*

• Action: %s
• Symbol: %s
• Exchange: %s
• Price: $%.4f
• Quantity: %.6f
• Value: $%.2f
• Time: %s`,
		emoji,
		strings.ToUpper(action),
		symbol,
		exchange,
		price,
		quantity,
		price*quantity,
		time.Now().Format("15:04:05"),
	)
}

// FormatDailyReport formats a daily P/L summary.
func FormatDailyReport(totalPnL float64, trades int, winRate float64) string {
	emoji := "📈"
	if totalPnL < 0 {
		emoji = "📉"
	}
	return fmt.Sprintf(`%s *Daily Report — %s*

• Trades: %d
• Win Rate: %.1f%%
• P/L: $%.2f

Keep going! 💪`,
		emoji,
		time.Now().Format("2006-01-02"),
		trades,
		winRate,
		totalPnL,
	)
}

// FormatPriceAlert formats a price alert notification.
func FormatPriceAlert(symbol string, price float64, direction string, threshold float64) string {
	emoji := "🚨"
	if direction == "above" {
		emoji = "📈"
	} else {
		emoji = "📉"
	}
	return fmt.Sprintf(`%s *Price Alert*

%s hit $%.4f (%s $%.4f threshold)`,
		emoji, symbol, price, direction, threshold)
}
