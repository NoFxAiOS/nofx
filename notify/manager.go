package notify

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	"strings"
)

// NotificationManager manages notifications for traders
type NotificationManager struct {
	notificationStore *store.NotificationStore
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(notificationStore *store.NotificationStore) *NotificationManager {
	return &NotificationManager{
		notificationStore: notificationStore,
	}
}

// GetWxPusherClient gets the WxPusher client for a specific trader
// Loads token from database and decrypts it automatically
func (nm *NotificationManager) GetWxPusherClient(userID, traderID string) (*WxPusherClient, error) {
	config, err := nm.notificationStore.GetByTraderID(userID, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification config: %w", err)
	}
	
	if config == nil || config.WxPusherToken == "" {
		return nil, fmt.Errorf("wxpusher token not configured")
	}
	
	// Token is automatically decrypted by EncryptedString.Scan()
	token := config.WxPusherToken.String()
	return NewWxPusherClient(token), nil
}

// NotifyDecision sends a notification about a trading decision
func (nm *NotificationManager) NotifyDecision(traderID, userID string, decision map[string]interface{}, opts ...MessageOption) error {
	// Get notification config
	config, err := nm.notificationStore.GetByTraderID(userID, traderID)
	if err != nil {
		return fmt.Errorf("failed to get notification config: %w", err)
	}

	// Check if notifications are enabled
	if config == nil || !config.IsEnabled {
		return nil
	}

	// Backfill defaults for legacy rows
	if !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
		config.EnableDecision = true
		config.EnableTradeOpen = true
		config.EnableTradeClose = true
	}

	if !config.EnableDecision {
		return nil
	}

	// Parse UIDs
	var uids []string
	if config.WxPusherUIDs != "" {
		if err := json.Unmarshal([]byte(config.WxPusherUIDs), &uids); err != nil {
			logger.Warnf("Failed to parse WxPusher UIDs: %v", err)
			return nil
		}
	}

	if len(uids) == 0 {
		return nil
	}

	// Get WxPusher client
	client, err := nm.GetWxPusherClient(userID, traderID)
	if err != nil {
		logger.Warnf("Failed to get wxpusher client: %v", err)
		return nil
	}

	// Format message
	content := formatDecisionMessage(decision)

	// Send message
	_, err = client.SendToUIDs(content, uids, opts...)
	if err != nil {
		logger.Warnf("Failed to send decision notification: %v", err)
		return nil
	}

	logger.Infof("📬 Notification sent for trader %s to %d users", traderID, len(uids))
	return nil
}

// NotifyTradeOpened sends a notification when a trade is opened
func (nm *NotificationManager) NotifyTradeOpened(traderID, userID, symbol string, side string, quantity, price float64) error {
	config, err := nm.notificationStore.GetByTraderID(userID, traderID)
	if err != nil || config == nil || !config.IsEnabled {
		return err
	}

	if !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
		config.EnableTradeOpen = true
	}

	if !config.EnableTradeOpen {
		return nil
	}

	var uids []string
	if config.WxPusherUIDs != "" {
		if err := json.Unmarshal([]byte(config.WxPusherUIDs), &uids); err != nil {
			return nil
		}
	}

	if len(uids) == 0 {
		return nil
	}

	client, err := nm.GetWxPusherClient(userID, traderID)
	if err != nil {
		return nil
	}

	content := fmt.Sprintf(
		"<h2>📈 Position Opened</h2><p><b>Symbol:</b> %s</p><p><b>Side:</b> %s</p><p><b>Quantity:</b> %.4f</p><p><b>Price:</b> %.2f</b></p>",
		symbol, strings.ToUpper(side), quantity, price,
	)

	_, err = client.SendToUIDs(content, uids, WithSummary(fmt.Sprintf("%s %s %.2f", symbol, side, price)))
	if err != nil {
		logger.Warnf("Failed to send trade opened notification: %v", err)
	}

	return nil
}

// NotifyTradeClosed sends a notification when a trade is closed
func (nm *NotificationManager) NotifyTradeClosed(traderID, userID, symbol string, side string, quantity, price, pnl float64) error {
	config, err := nm.notificationStore.GetByTraderID(userID, traderID)
	if err != nil || config == nil || !config.IsEnabled {
		return err
	}

	if !config.EnableDecision && !config.EnableTradeOpen && !config.EnableTradeClose {
		config.EnableTradeClose = true
	}

	if !config.EnableTradeClose {
		return nil
	}

	var uids []string
	if config.WxPusherUIDs != "" {
		if err := json.Unmarshal([]byte(config.WxPusherUIDs), &uids); err != nil {
			return nil
		}
	}

	if len(uids) == 0 {
		return nil
	}

	client, err := nm.GetWxPusherClient(userID, traderID)
	if err != nil {
		return nil
	}

	pnlStr := "🟢"
	if pnl < 0 {
		pnlStr = "🔴"
	}

	content := fmt.Sprintf(
		"<h2>📉 Position Closed</h2><p><b>Symbol:</b> %s</p><p><b>Side:</b> %s</p><p><b>Quantity:</b> %.4f</p><p><b>Price:</b> %.2f</p><p><b>PnL:</b> %s %.2f USDT</p>",
		symbol, strings.ToUpper(side), quantity, price, pnlStr, pnl,
	)

	_, err = client.SendToUIDs(content, uids, WithSummary(fmt.Sprintf("%s %s PnL: %.2f", symbol, side, pnl)))
	if err != nil {
		logger.Warnf("Failed to send trade closed notification: %v", err)
	}

	return nil
}

// formatDecisionMessage formats a decision into a beautiful HTML message like the web UI
func formatDecisionMessage(decision map[string]interface{}) string {
	var sb strings.Builder

	// Main container with dark theme
	sb.WriteString("<div style=\"background: linear-gradient(135deg, #1E2329 0%, #181C21 100%); border-radius: 12px; padding: 20px; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; color: #EAECEF;\">")

	// Header with cycle number
	sb.WriteString("<div style=\"display: flex; align-items: center; margin-bottom: 20px; gap: 12px;\">")
	sb.WriteString("<span style=\"font-size: 28px;\">🤖</span>")
	sb.WriteString("<div>")
	sb.WriteString("<div style=\"font-size: 18px; font-weight: 600; color: #EAECEF;\">AI Trading Decision</div>")
	if cycleNum, ok := decision["cycle"]; ok {
		sb.WriteString(fmt.Sprintf("<div style=\"font-size: 12px; color: #848E9C; margin-top: 2px;\">Cycle #%v</div>", cycleNum))
	}
	sb.WriteString("</div>")
	sb.WriteString("</div>")

	// Reasoning section
	if reasoning, ok := decision["reasoning"].(string); ok && reasoning != "" && reasoning != "{}" {
		sb.WriteString("<div style=\"background: rgba(240, 185, 11, 0.1); border-left: 4px solid #F0B90B; padding: 14px; margin-bottom: 16px; border-radius: 6px;\">")
		sb.WriteString("<div style=\"display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;\">")
		sb.WriteString("<div style=\"font-size: 12px; color: #F0B90B; font-weight: 600;\">💡 AI Reasoning</div>")
		sb.WriteString(fmt.Sprintf("<a href=\"javascript:void(0)\" style=\"font-size: 11px; color: #F0B90B; text-decoration: none; padding: 2px 6px; border-radius: 3px; border: 1px solid rgba(240, 185, 11, 0.4); cursor: pointer;\" onclick=\"var text = this.parentElement.nextElementSibling.innerText || this.parentElement.nextElementSibling.textContent; navigator.clipboard.writeText(text).then(() => {var btn = this; var origText = btn.innerText; btn.innerText = '✓ 已复制'; setTimeout(() => {btn.innerText = origText;}, 2000);}).catch(() => {alert('复制失败，请手动复制');});\">📋 复制</a>"))
		sb.WriteString("</div>")
		
		// If reasoning is longer than 300 chars, add preview + expand button
		if len(reasoning) > 300 {
			previewText := escapeHTML(reasoning[:300])
			fullText := preserveFormatting(escapeHTML(reasoning))
			sb.WriteString("<div style=\"font-size: 13px; color: #EAECEF; line-height: 1.6; white-space: pre-wrap; word-break: break-word;\">")
			sb.WriteString(previewText)
			sb.WriteString("...</div>")
			sb.WriteString("<div style=\"margin-top: 8px;\">")
			sb.WriteString(fmt.Sprintf("<a href=\"javascript:void(0)\" style=\"display: inline-block; padding: 6px 12px; background: rgba(240, 185, 11, 0.2); color: #F0B90B; border-radius: 4px; font-size: 12px; font-weight: 600; text-decoration: none; border: 1px solid rgba(240, 185, 11, 0.3);\" onclick=\"this.style.display='none'; this.nextElementSibling.style.display='block';\">🔼 展开完整内容</a>"))
			sb.WriteString(fmt.Sprintf("<div style=\"display: none; font-size: 13px; color: #EAECEF; line-height: 1.6; padding: 8px 0; white-space: pre-wrap; word-break: break-word;\">%s</div>", fullText))
			sb.WriteString("</div>")
		} else {
			sb.WriteString(fmt.Sprintf("<div style=\"font-size: 13px; color: #EAECEF; line-height: 1.6; white-space: pre-wrap; word-break: break-word;\">%s</div>", preserveFormatting(escapeHTML(reasoning))))
		}
		sb.WriteString("</div>")
	}

	// Parse and display trading actions
	if decisionJSON, ok := decision["decision"].(string); ok && decisionJSON != "" {
		var actions []map[string]interface{}
		if err := json.Unmarshal([]byte(decisionJSON), &actions); err == nil && len(actions) > 0 {
			sb.WriteString("<div style=\"margin-bottom: 16px;\">")
			for i, action := range actions {
				if i > 0 {
					sb.WriteString("<div style=\"height: 8px;\"></div>")
				}
				formatActionCard(&sb, action)
			}
			sb.WriteString("</div>")
		}
	}

	sb.WriteString("</div>")

	return sb.String()
}

// formatActionCard formats a single trading action as a card
func formatActionCard(sb *strings.Builder, action map[string]interface{}) {
	symbol := getStringField(action, "symbol", "UNKNOWN")
	actionType := getStringField(action, "action", "hold")
	confidence := getFloatField(action, "confidence", 0)

	// Determine colors and icons based on action type
	var actionColor, actionBg, actionLabel, actionIcon string
	switch actionType {
	case "open_long":
		actionColor = "#0ECB81"
		actionBg = "rgba(14, 203, 129, 0.15)"
		actionLabel = "LONG"
		actionIcon = "📈"
	case "open_short":
		actionColor = "#F6465D"
		actionBg = "rgba(246, 70, 93, 0.15)"
		actionLabel = "SHORT"
		actionIcon = "📉"
	case "close_long", "close_short":
		actionColor = "#F0B90B"
		actionBg = "rgba(240, 185, 11, 0.15)"
		actionLabel = "CLOSE"
		actionIcon = "💰"
	default:
		actionColor = "#848E9C"
		actionBg = "rgba(132, 142, 156, 0.15)"
		actionLabel = "HOLD"
		actionIcon = "⏸️"
	}

	// Card container
	sb.WriteString(fmt.Sprintf("<div style=\"background: linear-gradient(135deg, #232B32 0%, #1C2227 100%); border: 1px solid %s33; border-radius: 8px; padding: 14px; box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);\">", actionColor))

	// Card header
	sb.WriteString("<div style=\"display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px;\">")
	sb.WriteString("<div style=\"display: flex; align-items: center; gap: 10px; flex: 1;\">")
	sb.WriteString(fmt.Sprintf("<span style=\"font-size: 20px;\">%s</span>", actionIcon))
	sb.WriteString(fmt.Sprintf("<span style=\"font-family: 'Monaco', monospace; font-weight: 600; font-size: 14px; color: #EAECEF;\">%s</span>", truncateSymbol(symbol)))
	sb.WriteString(fmt.Sprintf("<span style=\"background: %s; color: %s; border: 1px solid %s55; padding: 3px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;\">%s</span>", actionBg, actionColor, actionColor, actionLabel))
	sb.WriteString("</div>")

	// Confidence badge
	if confidence > 0 {
		confColor := getConfidenceColor(confidence)
		sb.WriteString(fmt.Sprintf("<div style=\"background: %s22; color: %s; padding: 4px 8px; border-radius: 4px; font-size: 11px; font-weight: 600;\">%.0f%%</div>", confColor, confColor, confidence))
	}

	sb.WriteString("</div>")

	// Trading details for open positions
	if actionType == "open_long" || actionType == "open_short" {
		entryPrice := getFloatField(action, "price", 0)
		stopLoss := getFloatField(action, "stop_loss", 0)
		takeProfit := getFloatField(action, "take_profit", 0)
		leverage := getFloatField(action, "leverage", 1)

		sb.WriteString("<div style=\"display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; padding-top: 12px; border-top: 1px solid #2B3139;\">")

		// Entry Price
		sb.WriteString("<div style=\"text-align: center;\">")
		sb.WriteString("<div style=\"font-size: 11px; color: #848E9C; margin-bottom: 4px;\">Entry</div>")
		sb.WriteString(fmt.Sprintf("<div style=\"font-family: 'Monaco', monospace; font-size: 13px; font-weight: 600; color: #EAECEF;\">%s</div>", formatPrice(entryPrice)))
		sb.WriteString("</div>")

		// Stop Loss
		sb.WriteString("<div style=\"text-align: center;\">")
		sb.WriteString("<div style=\"font-size: 11px; color: #F6465D; margin-bottom: 4px;\">Stop Loss</div>")
		sb.WriteString(fmt.Sprintf("<div style=\"font-family: 'Monaco', monospace; font-size: 13px; font-weight: 600; color: #F6465D;\">%s</div>", formatPrice(stopLoss)))
		if stopLoss > 0 && entryPrice > 0 {
			pctChange := calcPercent(entryPrice, stopLoss, actionType == "open_long")
			sb.WriteString(fmt.Sprintf("<div style=\"font-size: 10px; color: #848E9C; margin-top: 2px;\">%s</div>", pctChange))
		}
		sb.WriteString("</div>")

		// Take Profit
		sb.WriteString("<div style=\"text-align: center;\">")
		sb.WriteString("<div style=\"font-size: 11px; color: #0ECB81; margin-bottom: 4px;\">Take Profit</div>")
		sb.WriteString(fmt.Sprintf("<div style=\"font-family: 'Monaco', monospace; font-size: 13px; font-weight: 600; color: #0ECB81;\">%s</div>", formatPrice(takeProfit)))
		if takeProfit > 0 && entryPrice > 0 {
			pctChange := calcPercent(entryPrice, takeProfit, true)
			sb.WriteString(fmt.Sprintf("<div style=\"font-size: 10px; color: #848E9C; margin-top: 2px;\">%s</div>", pctChange))
		}
		sb.WriteString("</div>")

		// Leverage
		sb.WriteString("<div style=\"text-align: center;\">")
		sb.WriteString("<div style=\"font-size: 11px; color: #848E9C; margin-bottom: 4px;\">Leverage</div>")
		sb.WriteString(fmt.Sprintf("<div style=\"font-family: 'Monaco', monospace; font-size: 13px; font-weight: 600; color: #F0B90B;\">%.0fx</div>", leverage))
		sb.WriteString("</div>")

		sb.WriteString("</div>")

		// Risk/Reward Ratio
		if stopLoss > 0 && takeProfit > 0 && entryPrice > 0 {
			slDist := math.Abs(entryPrice - stopLoss)
			tpDist := math.Abs(takeProfit - entryPrice)
			ratio := 0.0
			if slDist > 0 {
				ratio = tpDist / slDist
			}
			ratioColor := "#0ECB81"
			if ratio < 2 {
				ratioColor = "#F6465D"
			} else if ratio < 3 {
				ratioColor = "#F0B90B"
			}

			sb.WriteString("<div style=\"display: flex; align-items: center; justify-content: space-between; margin-top: 12px; padding-top: 12px; border-top: 1px solid #2B3139;\">")
			sb.WriteString("<span style=\"font-size: 11px; color: #848E9C;\">Risk/Reward</span>")
			sb.WriteString("<div style=\"display: flex; align-items: center; gap: 8px;\">")
			sb.WriteString(fmt.Sprintf("<div style=\"font-family: 'Monaco', monospace; font-size: 12px; font-weight: 600;\"><span style=\"color: #F6465D;\">1</span><span style=\"color: #848E9C;\">:</span><span style=\"color: %s;\">%.1f</span></div>", ratioColor, ratio))
			sb.WriteString(fmt.Sprintf("<div style=\"width: 50px; height: 6px; background: #2B3139; border-radius: 3px; overflow: hidden;\"><div style=\"width: %.0f%%; height: 100%%; background: %s; transition: width 0.3s;\"></div></div>", math.Min(ratio/5*100, 100), ratioColor))
			sb.WriteString("</div>")
			sb.WriteString("</div>")
		}
	}

	// Reasoning for this action
	if reasoning, ok := action["reasoning"].(string); ok && reasoning != "" {
		if len(reasoning) > 100 {
			previewText := escapeHTML(reasoning[:100])
			fullText := preserveFormatting(escapeHTML(reasoning))
			sb.WriteString(fmt.Sprintf("<div style=\"margin-top: 12px; padding-top: 12px; border-top: 1px solid #2B3139; font-size: 12px; color: #848E9C; line-height: 1.4; white-space: pre-wrap; word-break: break-word;\">💭 %s...", previewText))
			sb.WriteString(fmt.Sprintf("<a href=\"javascript:void(0)\" style=\"display: inline-block; margin-left: 4px; color: #F0B90B; text-decoration: none; font-weight: 600;\" onclick=\"if(this.textContent.indexOf('展开')>-1){this.textContent='[收起]'; this.nextElementSibling.style.display='block';}else{this.textContent='[展开]'; this.nextElementSibling.style.display='none';}\">[展开]</a>"))
			sb.WriteString(fmt.Sprintf("<a href=\"javascript:void(0)\" style=\"display: inline-block; margin-left: 4px; color: #F0B90B; text-decoration: none; font-weight: 600; font-size: 11px;\" onclick=\"var text = this.nextElementSibling.innerText || this.nextElementSibling.textContent; navigator.clipboard.writeText(text).then(() => {var btn = this; var origText = btn.innerText; btn.innerText = '✓'; setTimeout(() => {btn.innerText = origText;}, 1500);}).catch(() => {alert('复制失败');});\">[复制]</a>"))
			sb.WriteString(fmt.Sprintf("<div style=\"display: none; margin-top: 8px; padding: 8px; background: rgba(240, 185, 11, 0.05); border-radius: 4px; font-size: 12px; color: #EAECEF; line-height: 1.4; white-space: pre-wrap; word-break: break-word;\">%s</div></div>", fullText))
		} else {
			sb.WriteString(fmt.Sprintf("<div style=\"margin-top: 12px; padding-top: 12px; border-top: 1px solid #2B3139; font-size: 12px; color: #848E9C; line-height: 1.4; white-space: pre-wrap; word-break: break-word;\">💭 %s", preserveFormatting(escapeHTML(reasoning))))
			sb.WriteString(fmt.Sprintf("<a href=\"javascript:void(0)\" style=\"display: inline-block; margin-left: 8px; color: #F0B90B; text-decoration: none; font-weight: 600; font-size: 11px;\" onclick=\"var text = this.previousSibling.nodeValue; if(!text) { var parent = this.parentElement; text = parent.textContent.replace('[复制]', '').replace('💭 ', ''); } navigator.clipboard.writeText(text.trim()).then(() => {var btn = this; var origText = btn.innerText; btn.innerText = '✓'; setTimeout(() => {btn.innerText = origText;}, 1500);}).catch(() => {alert('复制失败');});\">[复制]</a></div>"))
		}
	}

	sb.WriteString("</div>")
}

// Helper functions for formatting
func getStringField(m map[string]interface{}, key string, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func getFloatField(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}

func truncateSymbol(symbol string) string {
	symbol = strings.TrimSuffix(symbol, "USDT")
	if len(symbol) > 10 {
		return symbol[:10]
	}
	return symbol
}

func truncateText(text string, maxLen int) string {
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}

func formatPrice(price float64) string {
	if price == 0 {
		return "-"
	}
	if price >= 1000 {
		return fmt.Sprintf("%.2f", price)
	}
	if price >= 1 {
		return fmt.Sprintf("%.4f", price)
	}
	return fmt.Sprintf("%.6f", price)
}

func calcPercent(entry, target float64, isLong bool) string {
	if entry == 0 || target == 0 {
		return "-"
	}
	pct := (target - entry) / entry * 100
	if !isLong {
		pct = -pct
	}
	sign := "+"
	if pct < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f%%", sign, pct)
}

func getConfidenceColor(conf float64) string {
	if conf >= 80 {
		return "#0ECB81"
	}
	if conf >= 60 {
		return "#F0B90B"
	}
	return "#F6465D"
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// preserveFormatting preserves line breaks and spacing in text for HTML display
// Replaces newlines with <br> tags and multiple spaces with &nbsp;
func preserveFormatting(s string) string {
	// Replace newlines with HTML line breaks
	s = strings.ReplaceAll(s, "\n", "<br>")
	// Replace multiple spaces with non-breaking spaces to preserve indentation
	// This is a simple approach - replace double spaces with space+nbsp
	s = strings.ReplaceAll(s, "  ", " &nbsp;")
	return s
}
