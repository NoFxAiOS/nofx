// Package store 提供统一的数据库存储层
// 所有数据库操作都应该通过这个包进行
package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"sync"

	_ "modernc.org/sqlite"
)

// Store 统一的数据存储接口
type Store struct {
	db *sql.DB

	// 子存储（延迟初始化）
	user         *UserStore
	aiModel      *AIModelStore
	exchange     *ExchangeStore
	trader       *TraderStore
	systemConfig *SystemConfigStore
	betaCode     *BetaCodeStore
	signalSource *SignalSourceStore
	decision     *DecisionStore
	backtest     *BacktestStore
	order        *OrderStore
	position     *PositionStore

	// 加密函数
	encryptFunc func(string) string
	decryptFunc func(string) string

	mu sync.RWMutex
}

// New 创建新的 Store 实例
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 配置
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// 启用外键约束
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键失败: %w", err)
	}

	// 使用 DELETE 模式（传统模式）以确保 Docker bind mount 兼容性
	// 注意：WAL 模式在 macOS Docker 下会导致数据同步问题
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置journal_mode失败: %w", err)
	}

	// 设置 synchronous=FULL
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置synchronous失败: %w", err)
	}

	// 设置 busy_timeout
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置busy_timeout失败: %w", err)
	}

	s := &Store{db: db}

	// 初始化所有表结构
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}

	// 初始化默认数据
	if err := s.initDefaultData(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	logger.Info("✅ 数据库已启用 DELETE 模式和 FULL 同步")
	return s, nil
}

// NewFromDB 从现有数据库连接创建 Store
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// SetCryptoFuncs 设置加密解密函数
func (s *Store) SetCryptoFuncs(encrypt, decrypt func(string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encryptFunc = encrypt
	s.decryptFunc = decrypt

	// 更新已初始化的子存储
	if s.aiModel != nil {
		s.aiModel.encryptFunc = encrypt
		s.aiModel.decryptFunc = decrypt
	}
	if s.exchange != nil {
		s.exchange.encryptFunc = encrypt
		s.exchange.decryptFunc = decrypt
	}
	if s.trader != nil {
		s.trader.decryptFunc = decrypt
	}
}

// initTables 初始化所有数据库表
func (s *Store) initTables() error {
	// 按依赖顺序初始化
	if err := s.User().initTables(); err != nil {
		return fmt.Errorf("初始化用户表失败: %w", err)
	}
	if err := s.AIModel().initTables(); err != nil {
		return fmt.Errorf("初始化AI模型表失败: %w", err)
	}
	if err := s.Exchange().initTables(); err != nil {
		return fmt.Errorf("初始化交易所表失败: %w", err)
	}
	if err := s.Trader().initTables(); err != nil {
		return fmt.Errorf("初始化交易员表失败: %w", err)
	}
	if err := s.SystemConfig().initTables(); err != nil {
		return fmt.Errorf("初始化系统配置表失败: %w", err)
	}
	if err := s.BetaCode().initTables(); err != nil {
		return fmt.Errorf("初始化内测码表失败: %w", err)
	}
	if err := s.SignalSource().initTables(); err != nil {
		return fmt.Errorf("初始化信号源表失败: %w", err)
	}
	if err := s.Decision().initTables(); err != nil {
		return fmt.Errorf("初始化决策日志表失败: %w", err)
	}
	if err := s.Backtest().initTables(); err != nil {
		return fmt.Errorf("初始化回测表失败: %w", err)
	}
	if err := s.Order().InitTables(); err != nil {
		return fmt.Errorf("初始化订单表失败: %w", err)
	}
	if err := s.Position().InitTables(); err != nil {
		return fmt.Errorf("初始化仓位表失败: %w", err)
	}
	return nil
}

// initDefaultData 初始化默认数据
func (s *Store) initDefaultData() error {
	if err := s.AIModel().initDefaultData(); err != nil {
		return err
	}
	if err := s.Exchange().initDefaultData(); err != nil {
		return err
	}
	if err := s.SystemConfig().initDefaultData(); err != nil {
		return err
	}
	return nil
}

// User 获取用户存储
func (s *Store) User() *UserStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil {
		s.user = &UserStore{db: s.db}
	}
	return s.user
}

// AIModel 获取AI模型存储
func (s *Store) AIModel() *AIModelStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiModel == nil {
		s.aiModel = &AIModelStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.aiModel
}

// Exchange 获取交易所存储
func (s *Store) Exchange() *ExchangeStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exchange == nil {
		s.exchange = &ExchangeStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.exchange
}

// Trader 获取交易员存储
func (s *Store) Trader() *TraderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trader == nil {
		s.trader = &TraderStore{
			db:          s.db,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.trader
}

// SystemConfig 获取系统配置存储
func (s *Store) SystemConfig() *SystemConfigStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.systemConfig == nil {
		s.systemConfig = &SystemConfigStore{db: s.db}
	}
	return s.systemConfig
}

// BetaCode 获取内测码存储
func (s *Store) BetaCode() *BetaCodeStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.betaCode == nil {
		s.betaCode = &BetaCodeStore{db: s.db}
	}
	return s.betaCode
}

// SignalSource 获取信号源存储
func (s *Store) SignalSource() *SignalSourceStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signalSource == nil {
		s.signalSource = &SignalSourceStore{db: s.db}
	}
	return s.signalSource
}

// Decision 获取决策日志存储
func (s *Store) Decision() *DecisionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decision == nil {
		s.decision = &DecisionStore{db: s.db}
	}
	return s.decision
}

// Backtest 获取回测数据存储
func (s *Store) Backtest() *BacktestStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backtest == nil {
		s.backtest = &BacktestStore{db: s.db}
	}
	return s.backtest
}

// Order 获取订单存储
func (s *Store) Order() *OrderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.order == nil {
		s.order = NewOrderStore(s.db)
	}
	return s.order
}

// Position 获取仓位存储
func (s *Store) Position() *PositionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.position == nil {
		s.position = NewPositionStore(s.db)
	}
	return s.position
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 获取底层数据库连接（仅用于兼容旧代码，逐步废弃）
// Deprecated: 使用 Store 的方法代替
func (s *Store) DB() *sql.DB {
	return s.db
}

// Transaction 执行事务
func (s *Store) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}
