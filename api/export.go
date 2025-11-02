package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"nofx/trader"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

// handleExportCSV 导出CSV
func (s *Server) handleExportCSV(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取导出数据类型
	dataType := c.DefaultQuery("type", "equity")

	var csvData []byte
	var filename string

	switch dataType {
	case "positions":
		csvData, err = exportPositionsCSV(t)
		filename = fmt.Sprintf("positions_%s_%s.csv", t.GetName(), time.Now().Format("20060102_150405"))
	case "decisions":
		csvData, err = exportDecisionsCSV(t)
		filename = fmt.Sprintf("decisions_%s_%s.csv", t.GetName(), time.Now().Format("20060102_150405"))
	case "equity":
		csvData, err = exportEquityHistoryCSV(t)
		filename = fmt.Sprintf("equity_history_%s_%s.csv", t.GetName(), time.Now().Format("20060102_150405"))
	case "statistics":
		csvData, err = exportStatisticsCSV(t)
		filename = fmt.Sprintf("statistics_%s_%s.csv", t.GetName(), time.Now().Format("20060102_150405"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的导出类型"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Data(http.StatusOK, "text/csv", csvData)
}

// handleExportPDF 导出PDF
func (s *Server) handleExportPDF(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取导出数据类型
	dataType := c.DefaultQuery("type", "full")

	var pdfData []byte
	var filename string

	switch dataType {
	case "full":
		pdfData, err = exportFullReportPDF(t)
		filename = fmt.Sprintf("report_%s_%s.pdf", t.GetName(), time.Now().Format("20060102_150405"))
	case "positions":
		pdfData, err = exportPositionsPDF(t)
		filename = fmt.Sprintf("positions_%s_%s.pdf", t.GetName(), time.Now().Format("20060102_150405"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的导出类型"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/pdf")
	c.Data(http.StatusOK, "application/pdf", pdfData)
}

// exportPositionsCSV 导出持仓CSV
func exportPositionsCSV(t *trader.AutoTrader) ([]byte, error) {
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %v", err)
	}

	var buf bytes.Buffer
	// 添加UTF-8 BOM以支持Excel正确显示中文
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)

	// 写入表头
	headers := []string{"交易对", "方向", "入场价格", "标记价格", "数量", "仓位价值(USDT)", "杠杆", "未实现盈亏(USDT)", "盈亏百分比(%)", "强平价格"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// 写入数据
	for _, pos := range positions {
		record := []string{
			getString(pos, "symbol"),
			getString(pos, "side"),
			floatToString(getFloat(pos, "entry_price"), 4),
			floatToString(getFloat(pos, "mark_price"), 4),
			floatToString(getFloat(pos, "quantity"), 4),
			floatToString(getFloat(pos, "quantity")*getFloat(pos, "mark_price"), 2),
			fmt.Sprintf("%.0fx", getFloat(pos, "leverage")),
			floatToString(getFloat(pos, "unrealized_pnl"), 2),
			floatToString(getFloat(pos, "unrealized_pnl_pct"), 2),
			floatToString(getFloat(pos, "liquidation_price"), 4),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportDecisionsCSV 导出决策日志CSV
func exportDecisionsCSV(t *trader.AutoTrader) ([]byte, error) {
	records, err := t.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		return nil, fmt.Errorf("获取决策日志失败: %v", err)
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)

	// 写入表头
	headers := []string{"周期", "时间", "是否成功", "净值(USDT)", "可用余额(USDT)", "保证金使用率(%)", "持仓数", "决策数量", "错误信息"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// 写入数据
	for _, rec := range records {
		accountState := rec.AccountState
		decisions := rec.Decisions

		csvRecord := []string{
			fmt.Sprintf("%d", rec.CycleNumber),
			rec.Timestamp.Format("2006-01-02 15:04:05"),
			boolToString(rec.Success),
			floatToString(accountState.TotalBalance, 2),
			floatToString(accountState.AvailableBalance, 2),
			floatToString(accountState.MarginUsedPct, 2),
			fmt.Sprintf("%d", accountState.PositionCount),
			fmt.Sprintf("%d", len(decisions)),
			rec.ErrorMessage,
		}
		if err := writer.Write(csvRecord); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportEquityHistoryCSV 导出收益历史CSV
func exportEquityHistoryCSV(t *trader.AutoTrader) ([]byte, error) {
	records, err := t.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		return nil, fmt.Errorf("获取历史数据失败: %v", err)
	}

	// 获取初始余额
	initialBalance := 0.0
	if status := t.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)

	// 写入表头
	headers := []string{"周期", "时间", "总权益(USDT)", "可用余额(USDT)", "总盈亏(USDT)", "盈亏百分比(%)", "持仓数", "保证金使用率(%)"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// 写入数据
	for _, rec := range records {
		accountState := rec.AccountState
		totalEquity := accountState.TotalBalance
		totalPnL := accountState.TotalUnrealizedProfit

		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (totalPnL / initialBalance) * 100
		}

		csvRecord := []string{
			fmt.Sprintf("%d", rec.CycleNumber),
			rec.Timestamp.Format("2006-01-02 15:04:05"),
			floatToString(totalEquity, 2),
			floatToString(accountState.AvailableBalance, 2),
			floatToString(totalPnL, 2),
			floatToString(totalPnLPct, 2),
			fmt.Sprintf("%d", accountState.PositionCount),
			floatToString(accountState.MarginUsedPct, 2),
		}
		if err := writer.Write(csvRecord); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportStatisticsCSV 导出统计数据CSV
func exportStatisticsCSV(t *trader.AutoTrader) ([]byte, error) {
	stats, err := t.GetDecisionLogger().GetStatistics()
	if err != nil {
		return nil, fmt.Errorf("获取统计数据失败: %v", err)
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buf)

	// 写入表头
	headers := []string{"统计项", "数值"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// 写入数据
	statsData := [][]string{
		{"总周期数", fmt.Sprintf("%d", stats.TotalCycles)},
		{"成功周期数", fmt.Sprintf("%d", stats.SuccessfulCycles)},
		{"失败周期数", fmt.Sprintf("%d", stats.FailedCycles)},
		{"开仓次数", fmt.Sprintf("%d", stats.TotalOpenPositions)},
		{"平仓次数", fmt.Sprintf("%d", stats.TotalClosePositions)},
	}

	for _, record := range statsData {
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		case int64:
			return float64(val)
		}
	}
	return 0.0
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func floatToString(f float64, precision int) string {
	return strconv.FormatFloat(f, 'f', precision, 64)
}

func boolToString(b bool) string {
	if b {
		return "成功"
	}
	return "失败"
}

// exportFullReportPDF exports a comprehensive trading report as PDF (English)
func exportFullReportPDF(t *trader.AutoTrader) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 20)

	// Title
	pdf.Cell(0, 10, "Trading Report - "+t.GetName())
	pdf.Ln(15)

	// Basic Info Section
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "Basic Information")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 6, "Trader Name: "+t.GetName())
	pdf.Ln(8)
	pdf.Cell(0, 6, "AI Model: "+t.GetAIModel())
	pdf.Ln(8)

	status := t.GetStatus()
	statusText := "Stopped"
	if getBool(status, "is_running") {
		statusText = "Running"
	}
	pdf.Cell(0, 6, "Status: "+statusText)
	pdf.Ln(8)
	pdf.Cell(0, 6, fmt.Sprintf("Cycles: %d", getInt(status, "call_count")))
	pdf.Ln(8)
	pdf.Cell(0, 6, fmt.Sprintf("Runtime: %d minutes", getInt(status, "runtime_minutes")))
	pdf.Ln(15)

	// Account Overview Section
	account, err := t.GetAccountInfo()
	if err == nil {
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 10, "Account Overview")
		pdf.Ln(10)

		pdf.SetFont("Arial", "", 12)
		pdf.Cell(0, 6, fmt.Sprintf("Total Equity: %.2f USDT", getFloat(account, "total_equity")))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Available Balance: %.2f USDT", getFloat(account, "available_balance")))
		pdf.Ln(8)

		pnl := getFloat(account, "total_pnl")
		pnlPct := getFloat(account, "total_pnl_pct")
		pdf.Cell(0, 6, fmt.Sprintf("Total P&L: %.2f USDT (%.2f%%)", pnl, pnlPct))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Position Count: %d", getInt(account, "position_count")))
		pdf.Ln(15)
	}

	// Current Positions Section
	positions, err := t.GetPositions()
	if err == nil && len(positions) > 0 {
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 10, "Current Positions")
		pdf.Ln(10)

		// Table header
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(25, 7, "Symbol", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Side", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "Entry", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "Mark", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Leverage", "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 7, "PnL (USDT)", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "PnL %", "1", 1, "C", true, 0, "")

		// Table data
		pdf.SetFont("Arial", "", 8)
		for i, pos := range positions {
			if i >= 10 { // Limit to 10 positions
				break
			}

			side := getString(pos, "side")
			sideText := "LONG"
			if side == "short" {
				sideText = "SHORT"
			}

			pdf.CellFormat(25, 6, getString(pos, "symbol"), "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 6, sideText, "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%.4f", getFloat(pos, "entry_price")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%.4f", getFloat(pos, "mark_price")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%.0fx", getFloat(pos, "leverage")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(35, 6, fmt.Sprintf("%.2f", getFloat(pos, "unrealized_pnl")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%.2f%%", getFloat(pos, "unrealized_pnl_pct")), "1", 1, "C", false, 0, "")
		}
		pdf.Ln(10)
	}

	// Statistics Section
	stats, err := t.GetDecisionLogger().GetStatistics()
	if err == nil {
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 10, "Statistics")
		pdf.Ln(10)

		pdf.SetFont("Arial", "", 12)
		pdf.Cell(0, 6, fmt.Sprintf("Total Cycles: %d", stats.TotalCycles))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Successful Cycles: %d", stats.SuccessfulCycles))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Failed Cycles: %d", stats.FailedCycles))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Total Open Positions: %d", stats.TotalOpenPositions))
		pdf.Ln(8)
		pdf.Cell(0, 6, fmt.Sprintf("Total Close Positions: %d", stats.TotalClosePositions))
		pdf.Ln(15)
	}

	// Footer
	pdf.SetY(-15)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(0, 10, fmt.Sprintf("Generated at %s", time.Now().Format("2006-01-02 15:04:05")))

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportPositionsPDF exports positions report as PDF (English)
func exportPositionsPDF(t *trader.AutoTrader) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape for wider table
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)

	// Title
	pdf.Cell(0, 10, "Positions Report - "+t.GetName())
	pdf.Ln(15)

	positions, err := t.GetPositions()
	if err != nil {
		return nil, err
	}

	if len(positions) == 0 {
		pdf.SetFont("Arial", "", 12)
		pdf.Cell(0, 10, "No positions")
	} else {
		// Table header
		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(30, 8, "Symbol", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 8, "Side", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Entry Price", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Mark Price", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Quantity", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 8, "Value (USDT)", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 8, "Leverage", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 8, "Unrealized PnL", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "PnL %", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Liq. Price", "1", 1, "C", true, 0, "")

		// Table data
		pdf.SetFont("Arial", "", 9)
		for _, pos := range positions {
			side := getString(pos, "side")
			sideText := "LONG"
			if side == "short" {
				sideText = "SHORT"
			}

			value := getFloat(pos, "quantity") * getFloat(pos, "mark_price")

			pdf.CellFormat(30, 7, getString(pos, "symbol"), "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 7, sideText, "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%.4f", getFloat(pos, "entry_price")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%.4f", getFloat(pos, "mark_price")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%.4f", getFloat(pos, "quantity")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(30, 7, fmt.Sprintf("%.2f", value), "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 7, fmt.Sprintf("%.0fx", getFloat(pos, "leverage")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(30, 7, fmt.Sprintf("%.2f", getFloat(pos, "unrealized_pnl")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%.2f%%", getFloat(pos, "unrealized_pnl_pct")), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%.4f", getFloat(pos, "liquidation_price")), "1", 1, "C", false, 0, "")
		}
	}

	// Footer
	pdf.SetY(-15)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(0, 10, fmt.Sprintf("Generated at %s", time.Now().Format("2006-01-02 15:04:05")))

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
