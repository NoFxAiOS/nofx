package proxy

import "strings"

// FixedIPProvider 固定IP列表提供者
type FixedIPProvider struct {
	ips []ProxyIP
}

// NewFixedIPProvider 创建固定IP列表提供者
func NewFixedIPProvider(proxyURLs []string) *FixedIPProvider {
	ips := make([]ProxyIP, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		// 简单解析代理URL
		// 格式: http://ip:port 或 socks5://user:pass@ip:port
		protocol := "http"
		if strings.HasPrefix(proxyURL, "socks5://") {
			protocol = "socks5"
			proxyURL = strings.TrimPrefix(proxyURL, "socks5://")
		} else if strings.HasPrefix(proxyURL, "http://") {
			proxyURL = strings.TrimPrefix(proxyURL, "http://")
		} else if strings.HasPrefix(proxyURL, "https://") {
			protocol = "https"
			proxyURL = strings.TrimPrefix(proxyURL, "https://")
		}

		ips = append(ips, ProxyIP{
			IP:       proxyURL,
			Protocol: protocol,
		})
	}

	return &FixedIPProvider{ips: ips}
}

func (p *FixedIPProvider) GetIPList() ([]ProxyIP, error) {
	return p.ips, nil
}

func (p *FixedIPProvider) RefreshIPList() ([]ProxyIP, error) {
	return p.ips, nil
}
