package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyManager 代理管理器
type ProxyManager struct {
	config   *Config
	provider IPProvider

	// IP池管理
	ipList      []ProxyIP
	blacklist   map[int]string   // ProxyID -> IP
	ipBlacklist map[string]int   // IP -> 剩余TTL
	mutex       sync.RWMutex     // 读写锁，保证线程安全

	// 刷新控制
	stopRefresh chan struct{}
}

var (
	globalProxyManager *ProxyManager
	once               sync.Once
)

// InitGlobalProxyManager 初始化全局代理管理器
func InitGlobalProxyManager(config *Config) error {
	var err error
	once.Do(func() {
		globalProxyManager, err = NewProxyManager(config)
		if err == nil && config.Enabled && config.RefreshInterval > 0 {
			globalProxyManager.StartAutoRefresh()
		}
	})
	return err
}

// GetGlobalProxyManager 获取全局代理管理器
func GetGlobalProxyManager() *ProxyManager {
	if globalProxyManager == nil {
		// 如果未初始化，使用默认配置（禁用代理）
		_ = InitGlobalProxyManager(&Config{Enabled: false})
	}
	return globalProxyManager
}

// NewProxyManager 创建代理管理器
func NewProxyManager(config *Config) (*ProxyManager, error) {
	if config == nil {
		config = &Config{Enabled: false}
	}

	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.BlacklistTTL == 0 {
		config.BlacklistTTL = 5 // 默认 TTL 为 5 次刷新
	}
	if config.RefreshInterval == 0 && config.Mode == "brightdata" {
		config.RefreshInterval = 30 * time.Minute // 默认 30 分钟刷新一次
	}

	m := &ProxyManager{
		config:      config,
		blacklist:   make(map[int]string),
		ipBlacklist: make(map[string]int),
		stopRefresh: make(chan struct{}),
	}

	// 如果未启用代理，直接返回
	if !config.Enabled {
		log.Printf("🌐 HTTP 代理未启用，使用直连")
		return m, nil
	}

	// 根据模式选择IP提供者
	switch config.Mode {
	case "single":
		// 单个代理模式
		if config.ProxyURL == "" {
			return nil, fmt.Errorf("single模式下必须配置proxy_url")
		}
		m.provider = NewSingleProxyProvider(config.ProxyURL)
		log.Printf("🌐 HTTP 代理已启用 (单代理模式): %s", config.ProxyURL)

	case "pool":
		// 代理池模式（固定列表）
		if len(config.ProxyList) == 0 {
			return nil, fmt.Errorf("pool模式下必须配置proxy_list")
		}
		m.provider = NewFixedIPProvider(config.ProxyList)
		log.Printf("🌐 HTTP 代理已启用 (代理池模式): %d个代理", len(config.ProxyList))

	case "brightdata":
		// Bright Data动态获取模式
		if config.BrightDataEndpoint == "" {
			return nil, fmt.Errorf("brightdata模式下必须配置brightdata_endpoint")
		}
		m.provider = NewBrightDataProvider(config.BrightDataEndpoint, config.BrightDataToken, config.BrightDataZone)
		log.Printf("🌐 HTTP 代理已启用 (Bright Data模式): %s", config.BrightDataEndpoint)

	default:
		// 默认使用single模式
		if config.ProxyURL == "" {
			return nil, fmt.Errorf("未知的proxy模式: %s", config.Mode)
		}
		m.provider = NewSingleProxyProvider(config.ProxyURL)
		log.Printf("🌐 HTTP 代理已启用 (默认模式): %s", config.ProxyURL)
	}

	// 初始化IP列表
	if err := m.RefreshIPList(); err != nil {
		return nil, fmt.Errorf("初始化IP列表失败: %w", err)
	}

	return m, nil
}

// RefreshIPList 刷新IP列表（线程安全）
func (m *ProxyManager) RefreshIPList() error {
	if m.provider == nil {
		return nil
	}

	ips, err := m.provider.RefreshIPList()
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清理黑名单，TTL倒计时
	validIPs := make([]ProxyIP, 0, len(ips))
	newBlacklist := make(map[int]string)

	for _, ip := range ips {
		if ttl, inBlacklist := m.ipBlacklist[ip.IP]; inBlacklist {
			// TTL 倒计时
			m.ipBlacklist[ip.IP] = ttl - 1
			if ttl > 0 {
				// 仍在黑名单中，跳过
				continue
			}
			// TTL 归零，从黑名单移除
			delete(m.ipBlacklist, ip.IP)
			log.Printf("✓ 代理IP已从黑名单恢复: %s", ip.IP)
		}
		validIPs = append(validIPs, ip)
	}

	m.ipList = validIPs
	m.blacklist = newBlacklist

	log.Printf("✓ 刷新代理IP列表: 总计%d个，黑名单%d个，可用%d个",
		len(ips), len(m.ipBlacklist), len(validIPs))

	return nil
}

// StartAutoRefresh 启动自动刷新
func (m *ProxyManager) StartAutoRefresh() {
	if m.config.RefreshInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(m.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.RefreshIPList(); err != nil {
					log.Printf("⚠️  自动刷新IP列表失败: %v", err)
				}
			case <-m.stopRefresh:
				return
			}
		}
	}()

	log.Printf("✓ 已启动代理IP自动刷新 (间隔: %v)", m.config.RefreshInterval)
}

// StopAutoRefresh 停止自动刷新
func (m *ProxyManager) StopAutoRefresh() {
	close(m.stopRefresh)
}

// getRandomProxy 随机获取一个可用代理（线程安全 - 读锁，确保不越界）
func (m *ProxyManager) getRandomProxy() (int, *ProxyIP, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.ipList) == 0 {
		return -1, nil, fmt.Errorf("代理IP列表为空")
	}

	// 找到所有未被黑名单的索引
	availableIndices := make([]int, 0, len(m.ipList))
	for i := range m.ipList {
		if _, inBlacklist := m.blacklist[i]; !inBlacklist {
			availableIndices = append(availableIndices, i)
		}
	}

	if len(availableIndices) == 0 {
		return -1, nil, fmt.Errorf("所有代理IP都在黑名单中")
	}

	// 随机选择一个（确保不越界）
	randomIdx := availableIndices[rand.Intn(len(availableIndices))]

	// 二次检查，确保索引有效（防御性编程）
	if randomIdx < 0 || randomIdx >= len(m.ipList) {
		return -1, nil, fmt.Errorf("代理索引越界: %d (总数: %d)", randomIdx, len(m.ipList))
	}

	return randomIdx, &m.ipList[randomIdx], nil
}

// buildProxyURL 构建代理URL
func (m *ProxyManager) buildProxyURL(ip *ProxyIP) string {
	if m.config.ProxyHost != "" && m.config.ProxyUser != "" {
		// 使用配置的代理主机和认证信息
		user := m.config.ProxyUser
		if m.config.ProxyUser != "" && ip.IP != "" {
			// 支持%s占位符替换IP
			user = fmt.Sprintf(m.config.ProxyUser, ip.IP)
		}

		protocol := ip.Protocol
		if protocol == "" {
			protocol = "http"
		}

		if m.config.ProxyPassword != "" {
			return fmt.Sprintf("%s://%s:%s@%s", protocol, user, m.config.ProxyPassword, m.config.ProxyHost)
		}
		return fmt.Sprintf("%s://%s@%s", protocol, user, m.config.ProxyHost)
	}

	// 直接使用IP信息
	return ip.IP
}

// GetProxyClient 获取代理客户端（线程安全）
func (m *ProxyManager) GetProxyClient() (*ProxyClient, error) {
	if !m.config.Enabled {
		// 未启用代理，返回普通HTTP客户端
		return &ProxyClient{
			ProxyID: -1, // -1 表示未使用代理
			IP:      "direct",
			Client: &http.Client{
				Timeout: m.config.Timeout,
			},
		}, nil
	}

	// 获取随机代理（使用读锁，确保不越界）
	proxyID, proxyIP, err := m.getRandomProxy()
	if err != nil {
		return nil, err
	}

	// 构建代理URL
	proxyURLStr := m.buildProxyURL(proxyIP)
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("解析代理URL失败: %w", err)
	}

	// 创建Transport
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &ProxyClient{
		ProxyID: proxyID,
		IP:      proxyIP.IP,
		Client: &http.Client{
			Transport: transport,
			Timeout:   m.config.Timeout,
		},
	}, nil
}

// AddBlacklist 将代理IP添加到黑名单（线程安全 - 写锁）
func (m *ProxyManager) AddBlacklist(proxyID int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查 proxyID 有效性，防止越界
	if proxyID < 0 || proxyID >= len(m.ipList) {
		log.Printf("⚠️  无效的 ProxyID: %d (有效范围: 0-%d)", proxyID, len(m.ipList)-1)
		return
	}

	ip := m.ipList[proxyID].IP
	m.blacklist[proxyID] = ip
	m.ipBlacklist[ip] = m.config.BlacklistTTL

	log.Printf("⚠️  代理IP已加入黑名单: %s (ProxyID: %d, TTL: %d)", ip, proxyID, m.config.BlacklistTTL)
}

// GetBlacklistStatus 获取黑名单状态（线程安全 - 读锁）
func (m *ProxyManager) GetBlacklistStatus() (total int, blacklisted int, available int) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	total = len(m.ipList)
	blacklisted = len(m.ipBlacklist)
	available = total - len(m.blacklist)
	return
}

// IsEnabled 检查代理是否启用
func IsEnabled() bool {
	return GetGlobalProxyManager().config.Enabled
}

// RefreshIPList 刷新全局代理IP列表
func RefreshIPList() error {
	return GetGlobalProxyManager().RefreshIPList()
}

// AddBlacklist 将代理IP添加到全局黑名单
func AddBlacklist(proxyID int) {
	GetGlobalProxyManager().AddBlacklist(proxyID)
}
