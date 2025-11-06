package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// BrightDataProvider Bright Data动态获取IP提供者
type BrightDataProvider struct {
	endpoint string
	token    string
	zone     string
	client   *http.Client
}

// NewBrightDataProvider 创建Bright Data IP提供者
func NewBrightDataProvider(endpoint, token, zone string) *BrightDataProvider {
	return &BrightDataProvider{
		endpoint: endpoint,
		token:    token,
		zone:     zone,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BrightDataIPList Bright Data API返回的IP列表结构
type BrightDataIPList struct {
	IPs []struct {
		IP      string                 `json:"ip"`
		Maxmind string                 `json:"maxmind"`
		Ext     map[string]interface{} `json:"ext"`
	} `json:"ips"`
}

func (p *BrightDataProvider) GetIPList() ([]ProxyIP, error) {
	return p.fetchIPList()
}

func (p *BrightDataProvider) RefreshIPList() ([]ProxyIP, error) {
	return p.fetchIPList()
}

func (p *BrightDataProvider) fetchIPList() ([]ProxyIP, error) {
	// 构建请求URL
	url := p.endpoint
	if p.zone != "" {
		url = fmt.Sprintf("%s?zone=%s", p.endpoint, p.zone)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置授权头
	if p.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	}

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取HTTP响应失败: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 解析JSON数据（支持Bright Data格式）
	var ipList BrightDataIPList
	if err := json.Unmarshal(body, &ipList); err != nil {
		return nil, fmt.Errorf("解析JSON数据失败: %w", err)
	}

	// 转换为ProxyIP列表
	result := make([]ProxyIP, 0, len(ipList.IPs))
	for _, ip := range ipList.IPs {
		result = append(result, ProxyIP{
			IP:       ip.IP,
			Protocol: "http",
			Ext:      ip.Ext,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("API返回的IP列表为空")
	}

	return result, nil
}
