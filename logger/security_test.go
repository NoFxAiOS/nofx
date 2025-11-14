package logger

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestLogDirectoryPermissions 测试日志目录的安全权限
func TestLogDirectoryPermissions(t *testing.T) {
	// 跳過 Windows（權限模型不同）
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	// 創建臨時測試目錄
	tempDir, err := ioutil.TempDir("", "logger_security_test_")
	if err != nil {
		t.Fatalf("創建臨時目錄失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "decision_logs")

	// 創建 DecisionLogger（會創建目錄並設置權限）
	logger := NewDecisionLogger(logDir)
	if logger == nil {
		t.Fatal("NewDecisionLogger 返回 nil")
	}

	// 驗證目錄存在
	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("日誌目錄不存在: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("logDir 不是目錄")
	}

	// 驗證目錄權限為 0700（只有所有者可訪問）
	expectedPerm := os.FileMode(0700)
	actualPerm := info.Mode().Perm()

	if actualPerm != expectedPerm {
		t.Errorf("目錄權限不正確: 期望 %o, 實際 %o", expectedPerm, actualPerm)
	}

	t.Logf("✅ 日誌目錄權限正確: %o", actualPerm)
}

// TestLogFilePermissions 测试日志文件的安全权限
func TestLogFilePermissions(t *testing.T) {
	// 跳過 Windows（權限模型不同）
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	// 創建臨時測試目錄
	tempDir, err := ioutil.TempDir("", "logger_security_test_")
	if err != nil {
		t.Fatalf("創建臨時目錄失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "decision_logs")

	// 創建 DecisionLogger
	logger := NewDecisionLogger(logDir)
	if logger == nil {
		t.Fatal("NewDecisionLogger 返回 nil")
	}

	// 創建測試記錄
	record := &DecisionRecord{
		ExecutionLog: []string{"Test log entry - security test"},
		Success:      true,
		AccountState: AccountSnapshot{
			TotalBalance:     1050.0,
			AvailableBalance: 1000.0,
			InitialBalance:   1000.0,
		},
	}

	// 寫入日誌文件
	err = logger.LogDecision(record)
	if err != nil {
		t.Fatalf("寫入日誌失敗: %v", err)
	}

	// 查找剛創建的日誌文件
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		t.Fatalf("讀取日誌目錄失敗: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("未找到日誌文件")
	}

	// 驗證文件權限為 0600（只有所有者可讀寫）
	expectedPerm := os.FileMode(0600)
	actualPerm := files[0].Mode().Perm()

	if actualPerm != expectedPerm {
		t.Errorf("文件權限不正確: 期望 %o, 實際 %o", expectedPerm, actualPerm)
	}

	t.Logf("✅ 日誌文件權限正確: %o", actualPerm)
}

// TestExistingDirectoryPermissionUpdate 测试已存在目录的权限更新
func TestExistingDirectoryPermissionUpdate(t *testing.T) {
	// 跳過 Windows（權限模型不同）
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	// 創建臨時測試目錄
	tempDir, err := ioutil.TempDir("", "logger_security_test_")
	if err != nil {
		t.Fatalf("創建臨時目錄失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "decision_logs")

	// 手動創建目錄並設置錯誤的權限（0755）
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("創建目錄失敗: %v", err)
	}

	// 驗證初始權限為 0755
	info, _ := os.Stat(logDir)
	initialPerm := info.Mode().Perm()
	if initialPerm != 0755 {
		t.Fatalf("初始權限設置失敗: %o", initialPerm)
	}

	t.Logf("初始目錄權限: %o", initialPerm)

	// 創建 DecisionLogger（應該修正權限為 0700）
	logger := NewDecisionLogger(logDir)
	if logger == nil {
		t.Fatal("NewDecisionLogger 返回 nil")
	}

	// 再次檢查權限
	info, err = os.Stat(logDir)
	if err != nil {
		t.Fatalf("獲取目錄信息失敗: %v", err)
	}

	updatedPerm := info.Mode().Perm()
	expectedPerm := os.FileMode(0700)

	if updatedPerm != expectedPerm {
		t.Errorf("權限未正確更新: 期望 %o, 實際 %o", expectedPerm, updatedPerm)
	}

	t.Logf("✅ 已存在目錄的權限已修正: %o → %o", initialPerm, updatedPerm)
}

// TestMultipleFilesPermissions 测试多个日志文件的权限一致性
func TestMultipleFilesPermissions(t *testing.T) {
	// 跳過 Windows（權限模型不同）
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	// 創建臨時測試目錄
	tempDir, err := ioutil.TempDir("", "logger_security_test_")
	if err != nil {
		t.Fatalf("創建臨時目錄失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logDir := filepath.Join(tempDir, "decision_logs")

	// 創建 DecisionLogger
	logger := NewDecisionLogger(logDir)
	if logger == nil {
		t.Fatal("NewDecisionLogger 返回 nil")
	}

	// 創建多個日誌記錄
	for i := 0; i < 5; i++ {
		record := &DecisionRecord{
			ExecutionLog: []string{"Test log entry"},
			Success:      true,
			AccountState: AccountSnapshot{
				TotalBalance:     float64(1050 + i*100),
				AvailableBalance: float64(1000 + i*100),
				InitialBalance:   float64(1000 + i*100),
			},
		}

		err = logger.LogDecision(record)
		if err != nil {
			t.Fatalf("寫入日誌 %d 失敗: %v", i, err)
		}
	}

	// 驗證所有文件的權限
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		t.Fatalf("讀取日誌目錄失敗: %v", err)
	}

	if len(files) != 5 {
		t.Fatalf("期望 5 個文件，實際 %d 個", len(files))
	}

	expectedPerm := os.FileMode(0600)
	for i, file := range files {
		actualPerm := file.Mode().Perm()
		if actualPerm != expectedPerm {
			t.Errorf("文件 %d 權限不正確: 期望 %o, 實際 %o", i, expectedPerm, actualPerm)
		}
	}

	t.Logf("✅ 所有 %d 個日誌文件的權限一致: %o", len(files), expectedPerm)
}
