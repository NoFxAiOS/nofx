package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"os"
	"strings"
)

// BetaCodeStore 内测码存储
type BetaCodeStore struct {
	db *sql.DB
}

func (s *BetaCodeStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// LoadFromFile 从文件加载内测码
func (s *BetaCodeStore) LoadFromFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取内测码文件失败: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO beta_codes (code) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			logger.Warnf("插入内测码 %s 失败: %v", code, err)
			continue
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.Infof("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
	return nil
}

// Validate 验证内测码是否有效
func (s *BetaCodeStore) Validate(code string) (bool, error) {
	var used bool
	err := s.db.QueryRow(`SELECT used FROM beta_codes WHERE code = ?`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return !used, nil
}

// Use 使用内测码
func (s *BetaCodeStore) Use(code, userEmail string) error {
	result, err := s.db.Exec(`
		UPDATE beta_codes SET used = 1, used_by = ?, used_at = CURRENT_TIMESTAMP
		WHERE code = ? AND used = 0
	`, userEmail, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("内测码无效或已被使用")
	}
	return nil
}

// GetStats 获取内测码统计
func (s *BetaCodeStore) GetStats() (total, used int, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
	if err != nil {
		return 0, 0, err
	}
	return total, used, nil
}
