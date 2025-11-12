package proxy

// SingleProxyProvider 单个代理提供者（不使用IP池）
type SingleProxyProvider struct {
	proxyURL string
}

// NewSingleProxyProvider 创建单个代理提供者
func NewSingleProxyProvider(proxyURL string) *SingleProxyProvider {
	return &SingleProxyProvider{proxyURL: proxyURL}
}

func (p *SingleProxyProvider) GetIPList() ([]ProxyIP, error) {
	return []ProxyIP{{IP: p.proxyURL}}, nil
}

func (p *SingleProxyProvider) RefreshIPList() ([]ProxyIP, error) {
	return p.GetIPList()
}
