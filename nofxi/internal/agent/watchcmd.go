package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// HandleWatchCommand processes /watch and /alert commands.
// Returns response text. Called from HandleMessage when intent is detected.
func (a *Agent) HandleWatchCommand(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))

	// /watch BTC
	if strings.HasPrefix(lower, "/watch") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return a.listWatched()
		}
		symbol := strings.ToUpper(parts[1])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		if a.monitor != nil {
			a.monitor.Watch(symbol)
			return fmt.Sprintf("👁️ Now watching *%s*. I'll track the price.", symbol)
		}
		return "⚠️ Market monitor not available."
	}

	// /unwatch BTC
	if strings.HasPrefix(lower, "/unwatch") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return "Usage: `/unwatch BTC`"
		}
		symbol := strings.ToUpper(parts[1])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		if a.monitor != nil {
			a.monitor.Unwatch(symbol)
			return fmt.Sprintf("🚫 Stopped watching *%s*.", symbol)
		}
		return "⚠️ Market monitor not available."
	}

	// /alert BTC above 100000
	if strings.HasPrefix(lower, "/alert") {
		parts := strings.Fields(text)
		if len(parts) < 4 {
			return "Usage: `/alert BTC above 100000` or `/alert ETH below 3000`"
		}
		symbol := strings.ToUpper(parts[1])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		direction := strings.ToLower(parts[2])
		if direction != "above" && direction != "below" {
			return "Direction must be `above` or `below`."
		}
		threshold, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return fmt.Sprintf("Invalid price: %s", parts[3])
		}

		if a.monitor != nil {
			a.monitor.Watch(symbol) // Ensure we're watching it
			a.monitor.AddAlert(symbol, direction, threshold)
			emoji := "📈"
			if direction == "below" {
				emoji = "📉"
			}
			return fmt.Sprintf("%s Alert set: *%s* %s $%.2f\nI'll notify you when it triggers.",
				emoji, symbol, direction, threshold)
		}
		return "⚠️ Market monitor not available."
	}

	// /price BTC
	if strings.HasPrefix(lower, "/price") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return "Usage: `/price BTC`"
		}
		symbol := strings.ToUpper(parts[1])
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		if a.monitor != nil {
			if snap, ok := a.monitor.GetSnapshot(symbol); ok && snap.LastPrice > 0 {
				return fmt.Sprintf("💰 *%s*: $%.4f\n_Updated: %s_",
					symbol, snap.LastPrice, snap.UpdatedAt.Format("15:04:05"))
			}
			// Not watching yet, watch and return message
			a.monitor.Watch(symbol)
			return fmt.Sprintf("👁️ Started watching *%s*. Price will be available in ~30s.", symbol)
		}
		return "⚠️ Market monitor not available."
	}

	return ""
}

func (a *Agent) listWatched() string {
	if a.monitor == nil {
		return "⚠️ Market monitor not available."
	}
	snaps := a.monitor.GetAllSnapshots()
	if len(snaps) == 0 {
		return "📭 Not watching any symbols. Use `/watch BTC` to start."
	}

	var sb strings.Builder
	sb.WriteString("👁️ *Watching*\n\n")
	for symbol, snap := range snaps {
		if snap.LastPrice > 0 {
			sb.WriteString(fmt.Sprintf("• *%s*: $%.4f (%s)\n",
				symbol, snap.LastPrice, snap.UpdatedAt.Format("15:04:05")))
		} else {
			sb.WriteString(fmt.Sprintf("• *%s*: waiting for data...\n", symbol))
		}
	}
	return sb.String()
}
