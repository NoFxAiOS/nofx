package logger

import (
	"os"
	"testing"
	"time"
)

func TestStructuredLogger_Create(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	logger, err := NewStructuredLogger(tmpDir)
	if err != nil {
		t.Fatalf("创建日志记录器失败: %v", err)
	}

	// 验证日志文件已创建
	logFile := tmpDir + "/news_config.log"
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("日志文件未被创建")
	}

	_ = logger // 避免未使用的变量警告
}

func TestStructuredLogger_LogCreate(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	newValue := map[string]interface{}{
		"user_id": "test-user",
		"enabled": true,
	}

	err := logger.LogCreate("test-user", newValue, 100*time.Millisecond, nil)
	if err != nil {
		t.Errorf("日志记录失败: %v", err)
	}
}

func TestStructuredLogger_LogUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	oldValue := map[string]interface{}{
		"enabled": true,
	}
	newValue := map[string]interface{}{
		"enabled": false,
	}

	err := logger.LogUpdate("test-user", oldValue, newValue, 50*time.Millisecond, nil)
	if err != nil {
		t.Errorf("日志记录失败: %v", err)
	}
}

func TestStructuredLogger_LogDelete(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	err := logger.LogDelete("test-user", 30*time.Millisecond, nil)
	if err != nil {
		t.Errorf("日志记录失败: %v", err)
	}
}

func TestStructuredLogger_LogWithError(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	testError := "配置已存在"
	err := logger.LogCreate("test-user", nil, 10*time.Millisecond, &testError)
	if err != nil {
		t.Errorf("日志记录失败: %v", err)
	}
}

func TestStructuredLogger_QueryLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	// 记录一些操作
	logger.LogCreate("user-1", nil, 100*time.Millisecond, nil)
	logger.LogCreate("user-2", nil, 100*time.Millisecond, nil)
	logger.LogDelete("user-1", 50*time.Millisecond, nil)

	// 查询user-1的日志
	logs, err := logger.QueryLogs("user-1", time.Time{}, time.Time{})
	if err != nil {
		t.Errorf("查询日志失败: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("期望2条日志，得到%d条", len(logs))
	}

	// 验证操作类型
	if logs[0].Operation != "create" && logs[0].Operation != "delete" {
		t.Error("日志中的操作类型不正确")
	}
}

func TestStructuredLogger_QueryLogs_ByDateRange(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	// 记录日志
	logger.LogCreate("test-user", nil, 100*time.Millisecond, nil)

	// 查询最近1小时的日志
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	logs, err := logger.QueryLogs("test-user", startTime, endTime)
	if err != nil {
		t.Errorf("查询日志失败: %v", err)
	}

	if len(logs) == 0 {
		t.Error("期望查询到日志")
	}
}

func TestStructuredLogger_GetLogStats(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewStructuredLogger(tmpDir)

	// 记录一些操作
	logger.LogCreate("user-1", nil, 100*time.Millisecond, nil)
	logger.LogCreate("user-1", nil, 100*time.Millisecond, nil)
	logger.LogUpdate("user-1", nil, nil, 100*time.Millisecond, nil)

	// 获取统计信息
	stats, err := logger.GetLogStats("user-1")
	if err != nil {
		t.Errorf("获取统计信息失败: %v", err)
	}

	if stats["total"] != 3 {
		t.Errorf("期望3条日志，得到%d条", stats["total"])
	}

	if stats["op_create"] != 2 {
		t.Errorf("期望2条create操作，得到%d条", stats["op_create"])
	}

	if stats["op_update"] != 1 {
		t.Errorf("期望1条update操作，得到%d条", stats["op_update"])
	}
}

func TestNewsConfigLog_Validation(t *testing.T) {
	// 测试日志结构的有效性
	log := &NewsConfigLog{
		Timestamp:    time.Now(),
		Level:        INFO,
		UserID:       "test-user",
		Operation:    "create",
		Status:       "success",
		Duration:     100,
	}

	// 验证字段
	if log.UserID != "test-user" {
		t.Error("日志的用户ID不正确")
	}

	if log.Level != INFO {
		t.Error("日志级别不正确")
	}

	if log.Operation != "create" {
		t.Error("操作类型不正确")
	}
}

func TestLogLevel_Values(t *testing.T) {
	// 验证日志级别的定义
	levels := []LogLevel{DEBUG, INFO, WARNING, ERROR}

	for _, level := range levels {
		if level == "" {
			t.Error("日志级别不能为空")
		}
	}
}
