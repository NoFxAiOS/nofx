package proxy

import (
	"net/http"
	"time"
)

// ProxyIP 代理IP信息
type ProxyIP struct {
	IP       string                 `json:"ip"`        // IP地址
	Port     string                 `json:"port"`      // 端口（可选）
	Username string                 `json:"username"`  // 用户名（可选）
	Password string                 `json:"password"`  // 密码（可选）
	Protocol string                 `json:"protocol"`  // 协议: http, https, socks5
	Ext      map[string]interface{} `json:"ext"`       // 扩展信息
}

// ProxyClient 代理客户端
type ProxyClient struct {
	ProxyID int            // IP池中的代理ID（索引）
	IP      string         // 使用的IP地址
	*http.Client          // HTTP客户端
}

// Config 代理配置
type Config struct {
	Enabled            bool          // 是否启用代理
	Mode               string        // 模式: "single", "pool", "brightdata"
	Timeout            time.Duration // 超时时间
	ProxyURL           string        // 单个代理地址 (single模式)
	ProxyList          []string      // 代理列表 (pool模式)
	BrightDataEndpoint string        // Bright Data接口地址 (brightdata模式)
	BrightDataToken    string        // Bright Data访问令牌 (brightdata模式)
	BrightDataZone     string        // Bright Data区域 (brightdata模式)
	ProxyHost          string        // 代理主机
	ProxyUser          string        // 代理用户名模板（支持%s占位符）
	ProxyPassword      string        // 代理密码
	RefreshInterval    time.Duration // IP列表刷新间隔
	BlacklistTTL       int           // 黑名单IP的TTL（刷新次数）
}
