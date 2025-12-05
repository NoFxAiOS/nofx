package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// StrategyStore 策略存储
type StrategyStore struct {
	db *sql.DB
}

// Strategy 策略配置
type Strategy struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`    // 是否激活（一个用户只能有一个激活的策略）
	IsDefault   bool      `json:"is_default"`   // 是否为系统默认策略
	Config      string    `json:"config"`       // JSON 格式的策略配置
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StrategyConfig 策略配置详情（JSON 结构）
type StrategyConfig struct {
	// 币种来源配置
	CoinSource CoinSourceConfig `json:"coin_source"`
	// 量化数据配置
	Indicators IndicatorConfig `json:"indicators"`
	// 自定义 Prompt（附加在最后）
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// 风险控制配置
	RiskControl RiskControlConfig `json:"risk_control"`
	// System Prompt 可编辑部分
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
}

// PromptSectionsConfig System Prompt 可编辑部分
type PromptSectionsConfig struct {
	// 角色定义（标题+描述）
	RoleDefinition string `json:"role_definition,omitempty"`
	// 交易频率认知
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// 开仓标准
	EntryStandards string `json:"entry_standards,omitempty"`
	// 决策流程
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig 币种来源配置
type CoinSourceConfig struct {
	// 来源类型: "static" | "coinpool" | "oi_top" | "mixed"
	SourceType string `json:"source_type"`
	// 静态币种列表（当 source_type = "static" 时使用）
	StaticCoins []string `json:"static_coins,omitempty"`
	// 是否使用 AI500 币种池
	UseCoinPool bool `json:"use_coin_pool"`
	// AI500 币种池最大数量
	CoinPoolLimit int `json:"coin_pool_limit,omitempty"`
	// AI500 币种池 API URL（策略级别配置）
	CoinPoolAPIURL string `json:"coin_pool_api_url,omitempty"`
	// 是否使用 OI Top
	UseOITop bool `json:"use_oi_top"`
	// OI Top 最大数量
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// OI Top API URL（策略级别配置）
	OITopAPIURL string `json:"oi_top_api_url,omitempty"`
}

// IndicatorConfig 指标配置
type IndicatorConfig struct {
	// K线配置
	Klines KlineConfig `json:"klines"`
	// 技术指标开关
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`          // 持仓量
	EnableFundingRate bool `json:"enable_funding_rate"` // 资金费率
	// EMA 周期配置
	EMAPeriods []int `json:"ema_periods,omitempty"` // 默认 [20, 50]
	// RSI 周期配置
	RSIPeriods []int `json:"rsi_periods,omitempty"` // 默认 [7, 14]
	// ATR 周期配置
	ATRPeriods []int `json:"atr_periods,omitempty"` // 默认 [14]
	// 外部数据源
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`
	// 量化数据源（资金流向、持仓变化、价格变化）
	EnableQuantData  bool   `json:"enable_quant_data"`            // 是否启用量化数据
	QuantDataAPIURL  string `json:"quant_data_api_url,omitempty"` // 量化数据 API 地址
}

// KlineConfig K线配置
type KlineConfig struct {
	// 主时间周期: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// 主时间周期 K 线数量
	PrimaryCount int `json:"primary_count"`
	// 长周期时间框架
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// 长周期 K 线数量
	LongerCount int `json:"longer_count,omitempty"`
	// 是否启用多时间框架分析
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// 选中的时间周期列表（新增：支持多周期选择）
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
}

// ExternalDataSource 外部数据源配置
type ExternalDataSource struct {
	Name        string            `json:"name"`         // 数据源名称
	Type        string            `json:"type"`         // 类型: "api" | "webhook"
	URL         string            `json:"url"`          // API URL
	Method      string            `json:"method"`       // HTTP 方法
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON 数据路径
	RefreshSecs int               `json:"refresh_secs,omitempty"` // 刷新间隔（秒）
}

// RiskControlConfig 风险控制配置
type RiskControlConfig struct {
	// 最大持仓数量
	MaxPositions int `json:"max_positions"`
	// BTC/ETH 最大杠杆
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// 山寨币最大杠杆
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`
	// 最小风险回报比
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// 最大保证金使用率
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// 单币种最大仓位比例（相对账户净值）
	MaxPositionRatio float64 `json:"max_position_ratio"`
	// 最小开仓金额（USDT）
	MinPositionSize float64 `json:"min_position_size"`
	// 最小信心度
	MinConfidence int `json:"min_confidence"`
}

func (s *StrategyStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 0,
			is_default BOOLEAN DEFAULT 0,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 创建索引
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_user_id ON strategies(user_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_is_active ON strategies(is_active)`)

	// 触发器：更新时自动更新 updated_at
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_strategies_updated_at
		AFTER UPDATE ON strategies
		BEGIN
			UPDATE strategies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)

	return err
}

func (s *StrategyStore) initDefaultData() error {
	// 检查是否已有默认策略
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM strategies WHERE is_default = 1`).Scan(&count)
	if count > 0 {
		return nil
	}

	// 创建系统默认策略
	defaultConfig := StrategyConfig{
		CoinSource: CoinSourceConfig{
			SourceType:     "coinpool",
			UseCoinPool:    true,
			CoinPoolLimit:  30,
			CoinPoolAPIURL: "http://nofxaios.com:30006/api/ai500/list?auth=cm_568c67eae410d912c54c",
			UseOITop:       false,
			OITopLimit:     0,
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableEMA:         true,
			EnableMACD:        true,
			EnableRSI:         true,
			EnableATR:         true,
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
			EnableQuantData:  true,
			QuantDataAPIURL:  "http://nofxaios.com:30006/api/coin/{symbol}?include=netflow,oi,price&auth=cm_568c67eae410d912c54c",
		},
		RiskControl: RiskControlConfig{
			MaxPositions:       3,
			BTCETHMaxLeverage:  5,
			AltcoinMaxLeverage: 5,
			MinRiskRewardRatio: 3.0,
			MaxMarginUsage:     0.9,
			MaxPositionRatio:   1.5,
			MinPositionSize:    12,
			MinConfidence:      75,
		},
		PromptSections: PromptSectionsConfig{
			RoleDefinition: `# 你是专业的加密货币交易AI

你的任务是根据提供的市场数据做出交易决策。你是一位经验丰富的量化交易员，擅长技术分析和风险管理。`,
			TradingFrequency: `# ⏱️ 交易频率认知

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时>2笔 = 过度交易
- 单笔持仓时间≥30-60分钟
如果你发现自己每个周期都在交易 → 标准过低；若持仓<30分钟就平仓 → 过于急躁。`,
			EntryStandards: `# 🎯 开仓标准（严格）

只在多重信号共振时开仓。自由运用任何有效的分析方法，避免单一指标、信号矛盾、横盘震荡、刚平仓即重启等低质量行为。`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否该止盈/止损
2. 扫描候选币 + 多时间框 → 是否存在强信号
3. 先写思维链，再输出结构化JSON`,
		},
	}

	configJSON, _ := json.Marshal(defaultConfig)

	_, err := s.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, is_active, is_default, config)
		VALUES ('default', 'system', '默认山寨策略', '系统默认的山寨币交易策略，使用 AI500 币种池，包含完整的技术指标', 0, 1, ?)
	`, string(configJSON))

	return err
}

// Create 创建策略
func (s *StrategyStore) Create(strategy *Strategy) error {
	_, err := s.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, is_active, is_default, config)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, strategy.ID, strategy.UserID, strategy.Name, strategy.Description, strategy.IsActive, strategy.IsDefault, strategy.Config)
	return err
}

// Update 更新策略
func (s *StrategyStore) Update(strategy *Strategy) error {
	_, err := s.db.Exec(`
		UPDATE strategies SET
			name = ?, description = ?, config = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, strategy.Name, strategy.Description, strategy.Config, strategy.ID, strategy.UserID)
	return err
}

// Delete 删除策略
func (s *StrategyStore) Delete(userID, id string) error {
	// 不允许删除系统默认策略
	var isDefault bool
	s.db.QueryRow(`SELECT is_default FROM strategies WHERE id = ?`, id).Scan(&isDefault)
	if isDefault {
		return fmt.Errorf("不能删除系统默认策略")
	}

	_, err := s.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// List 获取用户的策略列表
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	// 获取用户自己的策略 + 系统默认策略
	rows, err := s.db.Query(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? OR is_default = 1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []*Strategy
	for rows.Next() {
		var st Strategy
		var createdAt, updatedAt string
		err := rows.Scan(
			&st.ID, &st.UserID, &st.Name, &st.Description,
			&st.IsActive, &st.IsDefault, &st.Config,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		strategies = append(strategies, &st)
	}
	return strategies, nil
}

// Get 获取单个策略
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE id = ? AND (user_id = ? OR is_default = 1)
	`, id, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetActive 获取用户当前激活的策略
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? AND is_active = 1
	`, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		// 没有激活的策略，返回系统默认策略
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetDefault 获取系统默认策略
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE is_default = 1
		LIMIT 1
	`).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// SetActive 设置激活策略（会先取消其他策略的激活状态）
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先取消该用户所有策略的激活状态
	_, err = tx.Exec(`UPDATE strategies SET is_active = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// 激活指定策略
	_, err = tx.Exec(`UPDATE strategies SET is_active = 1 WHERE id = ? AND (user_id = ? OR is_default = 1)`, strategyID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Duplicate 复制策略（用于基于默认策略创建自定义策略）
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// 获取源策略
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("获取源策略失败: %w", err)
	}

	// 创建新策略
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "基于 [" + source.Name + "] 创建",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig 解析策略配置 JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("解析策略配置失败: %w", err)
	}
	return &config, nil
}

// SetConfig 设置策略配置
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化策略配置失败: %w", err)
	}
	s.Config = string(data)
	return nil
}
