package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	DEBUG   LogLevel = "DEBUG"
	INFO    LogLevel = "INFO"
	WARNING LogLevel = "WARNING"
	ERROR   LogLevel = "ERROR"
)

// NewsConfigLog 新闻配置操作日志
type NewsConfigLog struct {
	Timestamp    time.Time `json:"timestamp"`
	Level        LogLevel  `json:"level"`
	UserID       string    `json:"user_id"`
	Operation    string    `json:"operation"` // create, update, delete, fetch
	ResourceID   int       `json:"resource_id,omitempty"`
	OldValue     string    `json:"old_value,omitempty"` // JSON格式
	NewValue     string    `json:"new_value,omitempty"` // JSON格式
	Status       string    `json:"status"`              // success, failed
	ErrorMessage string    `json:"error_message,omitempty"`
	Duration     int64     `json:"duration_ms"` // 操作耗时
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
}

// StructuredLogger 结构化日志记录器
type StructuredLogger struct {
	logDir string
	logger *log.Logger
}

// NewStructuredLogger 创建新的结构化日志记录器
func NewStructuredLogger(logDir string) (*StructuredLogger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件
	logFile := filepath.Join(logDir, "news_config.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	logger := log.New(file, "", 0) // 不使用默认前缀，我们使用JSON格式

	return &StructuredLogger{
		logDir: logDir,
		logger: logger,
	}, nil
}

// LogNewsConfigOperation 记录新闻配置操作
func (sl *StructuredLogger) LogNewsConfigOperation(log *NewsConfigLog) error {
	// 自动填充时间戳
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// 转换为JSON
	jsonBytes, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	// 记录到文件
	sl.logger.Println(string(jsonBytes))

	// 同时输出到标准输出（用于开发调试）
	sl.printToConsole(log)

	return nil
}

// printToConsole 打印到控制台（格式化输出）
func (sl *StructuredLogger) printToConsole(log *NewsConfigLog) {
	// 根据级别选择输出格式
	emoji := ""
	switch log.Level {
	case DEBUG:
		emoji = "🔍"
	case INFO:
		emoji = "ℹ️"
	case WARNING:
		emoji = "⚠️"
	case ERROR:
		emoji = "❌"
	}

	status := "✓"
	if log.Status == "failed" {
		status = "✗"
	}

	fmt.Printf("%s [%s] %s %s (user: %s, duration: %dms)\n",
		emoji,
		log.Level,
		status,
		log.Operation,
		log.UserID,
		log.Duration,
	)

	if log.ErrorMessage != "" {
		fmt.Printf("  错误: %s\n", log.ErrorMessage)
	}
}

// LogCreate 记录创建操作
func (sl *StructuredLogger) LogCreate(userID string, newValue interface{}, duration time.Duration, err error) error {
	newValueJSON, _ := json.Marshal(newValue)

	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	return sl.LogNewsConfigOperation(&NewsConfigLog{
		Level:        INFO,
		UserID:       userID,
		Operation:    "create",
		NewValue:     string(newValueJSON),
		Status:       status,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	})
}

// LogUpdate 记录更新操作
func (sl *StructuredLogger) LogUpdate(userID string, oldValue, newValue interface{}, duration time.Duration, err error) error {
	oldValueJSON, _ := json.Marshal(oldValue)
	newValueJSON, _ := json.Marshal(newValue)

	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	return sl.LogNewsConfigOperation(&NewsConfigLog{
		Level:        INFO,
		UserID:       userID,
		Operation:    "update",
		OldValue:     string(oldValueJSON),
		NewValue:     string(newValueJSON),
		Status:       status,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	})
}

// LogDelete 记录删除操作
func (sl *StructuredLogger) LogDelete(userID string, duration time.Duration, err error) error {
	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	return sl.LogNewsConfigOperation(&NewsConfigLog{
		Level:        INFO,
		UserID:       userID,
		Operation:    "delete",
		Status:       status,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	})
}

// LogFetch 记录查询操作
func (sl *StructuredLogger) LogFetch(userID string, duration time.Duration, err error) error {
	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	return sl.LogNewsConfigOperation(&NewsConfigLog{
		Level:        DEBUG,
		UserID:       userID,
		Operation:    "fetch",
		Status:       status,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	})
}

// QueryLogs 查询日志（按日期范围和用户）
func (sl *StructuredLogger) QueryLogs(userID string, startTime, endTime time.Time) ([]NewsConfigLog, error) {
	logFile := filepath.Join(sl.logDir, "news_config.log")

	// 读取日志文件
	content, err := os.ReadFile(logFile)
	if err != nil {
		return nil, fmt.Errorf("读取日志文件失败: %w", err)
	}

	var logs []NewsConfigLog
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var log NewsConfigLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue // 跳过无效的JSON行
		}

		// 按条件过滤
		if userID != "" && log.UserID != userID {
			continue
		}
		if !startTime.IsZero() && log.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && log.Timestamp.After(endTime) {
			continue
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetLogStats 获取日志统计信息
func (sl *StructuredLogger) GetLogStats(userID string) (map[string]int, error) {
	logs, err := sl.QueryLogs(userID, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}

	stats := make(map[string]int)
	stats["total"] = len(logs)

	for _, log := range logs {
		// 统计操作类型
		key := fmt.Sprintf("op_%s", log.Operation)
		stats[key]++

		// 统计状态
		key = fmt.Sprintf("status_%s", log.Status)
		stats[key]++
	}

	return stats, nil
}
