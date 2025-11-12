package proxy

import (
	"log"
	"net/http"
	"time"
)

// --- 便捷函数（直接使用全局管理器） ---

// GetProxyHTTPClient 获取代理 HTTP 客户端（返回 ProxyClient，包含 ProxyID）
func GetProxyHTTPClient() (*ProxyClient, error) {
	return GetGlobalProxyManager().GetProxyClient()
}

// NewHTTPClient 创建一个新的HTTP客户端（使用全局代理配置）
// 注意：不返回 ProxyID，如需 ProxyID 请使用 GetProxyHTTPClient()
func NewHTTPClient() *http.Client {
	client, err := GetGlobalProxyManager().GetProxyClient()
	if err != nil {
		log.Printf("⚠️  获取代理客户端失败，使用直连: %v", err)
		return &http.Client{Timeout: 30 * time.Second}
	}
	return client.Client
}

// NewHTTPClientWithTimeout 创建一个新的HTTP客户端并指定超时时间
// 注意：不返回 ProxyID，如需 ProxyID 请使用 GetProxyHTTPClient()
func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	client, err := GetGlobalProxyManager().GetProxyClient()
	if err != nil {
		log.Printf("⚠️  获取代理客户端失败，使用直连: %v", err)
		return &http.Client{Timeout: timeout}
	}
	client.Client.Timeout = timeout
	return client.Client
}

// GetTransport 获取HTTP Transport
func GetTransport() *http.Transport {
	client, err := GetGlobalProxyManager().GetProxyClient()
	if err != nil {
		log.Printf("⚠️  获取代理客户端失败，使用直连: %v", err)
		return &http.Transport{}
	}
	return client.Client.Transport.(*http.Transport)
}