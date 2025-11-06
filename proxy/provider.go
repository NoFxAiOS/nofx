package proxy

// IPProvider IP提供者接口
type IPProvider interface {
	// GetIPList 获取IP列表
	GetIPList() ([]ProxyIP, error)

	// RefreshIPList 刷新IP列表（可选实现）
	RefreshIPList() ([]ProxyIP, error)
}
