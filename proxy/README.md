# HTTP 代理模块

## 概述

这是一个高度解耦的HTTP代理管理模块，专为解决高频API请求被限流/封禁问题而设计。支持单代理、代理池和动态IP获取三种模式，提供线程安全的IP轮换和智能黑名单管理机制。

## 功能特性

- ✅ **三种工作模式**：单代理、固定代理池、Bright Data API动态获取
- ✅ **线程安全**：所有操作使用读写锁保护，支持并发访问
- ✅ **智能黑名单**：失败的代理IP手动加入黑名单，TTL机制自动恢复
- ✅ **自动刷新**：支持定时刷新代理IP列表（默认30分钟）
- ✅ **随机轮换**：从可用IP池中随机选择，避免单点压力
- ✅ **防越界保护**：多层数组边界检查，确保运行时安全
- ✅ **可选启用**：未配置或禁用时自动使用直连，不影响独立客户

## 架构设计

```
proxy/
├── README.md                    # 本文档
├── types.go                     # 核心数据结构定义
├── provider.go                  # IP提供者接口定义
├── single_provider.go           # 单代理实现
├── fixed_provider.go            # 固定代理池实现
├── brightdata_provider.go       # Bright Data API实现
└── proxy_manager.go             # 代理管理器（核心逻辑）
```

### 设计原则

1. **接口抽象**：通过 `IPProvider` 接口实现不同代理源的统一管理
2. **策略模式**：三种Provider实现可灵活切换
3. **单例模式**：全局ProxyManager确保资源统一管理
4. **防御性编程**：多层边界检查，优雅处理异常情况

## 配置说明

在 `config.json` 中添加 `proxy` 配置段：

```json
{
  "proxy": {
    "enabled": true,
    "mode": "single",
    "timeout": 30,
    "proxy_url": "http://127.0.0.1:7890",
    "proxy_list": [],
    "brightdata_endpoint": "",
    "brightdata_token": "",
    "brightdata_zone": "",
    "proxy_host": "",
    "proxy_user": "",
    "proxy_password": "",
    "refresh_interval": 1800,
    "blacklist_ttl": 5
  }
}
```

### 配置字段详解

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `enabled` | bool | 是 | 是否启用代理（false时使用直连） |
| `mode` | string | 是 | 代理模式：`single`/`pool`/`brightdata` |
| `timeout` | int | 否 | HTTP请求超时时间（秒），默认30 |
| `proxy_url` | string | single模式必填 | 单个代理地址，如 `http://127.0.0.1:7890` |
| `proxy_list` | []string | pool模式必填 | 代理列表，支持 `http://`、`https://`、`socks5://` |
| `brightdata_endpoint` | string | brightdata模式必填 | Bright Data API端点 |
| `brightdata_token` | string | brightdata模式可选 | Bright Data访问令牌 |
| `brightdata_zone` | string | brightdata模式可选 | Bright Data区域参数 |
| `proxy_host` | string | 否 | 代理主机（用于认证代理） |
| `proxy_user` | string | 否 | 代理用户名模板，支持 `%s` 占位符替换IP |
| `proxy_password` | string | 否 | 代理密码 |
| `refresh_interval` | int | 否 | IP列表刷新间隔（秒），brightdata模式默认1800（30分钟） |
| `blacklist_ttl` | int | 否 | 黑名单IP的TTL（刷新次数），默认5 |

## 使用方法

### 1. 初始化代理管理器

在 `main.go` 或初始化代码中：

```go
import (
    "nofx/proxy"
    "time"
)

// 方式1：使用配置结构体初始化
proxyConfig := &proxy.Config{
    Enabled: true,
    Mode: "single",
    Timeout: 30 * time.Second,
    ProxyURL: "http://127.0.0.1:7890",
    BlacklistTTL: 5,
}

err := proxy.InitGlobalProxyManager(proxyConfig)
if err != nil {
    log.Fatalf("初始化代理管理器失败: %v", err)
}
```

### 2. 获取代理HTTP客户端

在需要发送HTTP请求的地方：

```go
// 获取代理客户端（包含ProxyID用于黑名单管理）
proxyClient, err := proxy.GetProxyHTTPClient()
if err != nil {
    log.Printf("获取代理客户端失败: %v", err)
    return
}

// 使用代理客户端发送请求
resp, err := proxyClient.Client.Get("https://api.example.com/data")
if err != nil {
    // 请求失败，将此代理加入黑名单
    proxy.AddBlacklist(proxyClient.ProxyID)
    log.Printf("请求失败，代理IP %s 已加入黑名单", proxyClient.IP)
    return
}
defer resp.Body.Close()

// 处理响应...
```

### 3. 黑名单管理

```go
// 添加失败的代理到黑名单
proxy.AddBlacklist(proxyClient.ProxyID)

// 获取黑名单状态
total, blacklisted, available := proxy.GetGlobalProxyManager().GetBlacklistStatus()
log.Printf("代理状态: 总计%d个，黑名单%d个，可用%d个", total, blacklisted, available)
```

### 4. 手动刷新IP列表

```go
err := proxy.RefreshIPList()
if err != nil {
    log.Printf("刷新IP列表失败: %v", err)
}
```

### 5. 检查代理是否启用

```go
if proxy.IsEnabled() {
    log.Println("代理已启用")
} else {
    log.Println("代理未启用，使用直连")
}
```

## 三种模式详解

### Mode 1: Single（单代理模式）

适用场景：本地代理工具（如Clash、V2Ray）或单个固定代理服务器

```json
{
  "proxy": {
    "enabled": true,
    "mode": "single",
    "proxy_url": "http://127.0.0.1:7890"
  }
}
```

特点：
- 简单直接，适合本地开发和测试
- 所有请求通过同一个代理
- 不需要刷新和轮换

### Mode 2: Pool（代理池模式）

适用场景：拥有多个固定代理服务器，需要轮换使用

```json
{
  "proxy": {
    "enabled": true,
    "mode": "pool",
    "proxy_list": [
      "http://proxy1.example.com:8080",
      "http://user:pass@proxy2.example.com:8080",
      "socks5://proxy3.example.com:1080"
    ],
    "blacklist_ttl": 5
  }
}
```

特点：
- 支持多协议：HTTP、HTTPS、SOCKS5
- 随机选择代理，分散请求压力
- 失败的代理自动加入黑名单
- 黑名单IP经过TTL次刷新后自动恢复

### Mode 3: BrightData（动态IP模式）

适用场景：使用Bright Data等提供API的动态代理服务

```json
{
  "proxy": {
    "enabled": true,
    "mode": "brightdata",
    "brightdata_endpoint": "https://api.brightdata.com/zones/get_ips",
    "brightdata_token": "your_api_token",
    "brightdata_zone": "residential",
    "proxy_host": "brd.superproxy.io:22225",
    "proxy_user": "brd-customer-xxx-zone-residential-ip-%s",
    "proxy_password": "your_password",
    "refresh_interval": 1800,
    "blacklist_ttl": 5
  }
}
```

特点：
- 从API动态获取可用IP列表
- 自动定时刷新（默认30分钟）
- 支持用户名模板（`%s` 替换为IP地址）
- 黑名单TTL机制避免频繁切换

**用户名模板说明**：
```
proxy_user: "brd-customer-xxx-zone-residential-ip-%s"
                                                    ↑
                                             自动替换为IP地址
```

## 核心API

### 全局函数

```go
// 初始化全局代理管理器（只执行一次）
func InitGlobalProxyManager(config *Config) error

// 获取全局代理管理器实例
func GetGlobalProxyManager() *ProxyManager

// 获取代理HTTP客户端（包含ProxyID和IP信息）
func GetProxyHTTPClient() (*ProxyClient, error)

// 将代理IP添加到黑名单
func AddBlacklist(proxyID int)

// 刷新IP列表
func RefreshIPList() error

// 检查代理是否启用
func IsEnabled() bool
```

### ProxyManager 方法

```go
// 获取代理客户端
func (m *ProxyManager) GetProxyClient() (*ProxyClient, error)

// 刷新IP列表
func (m *ProxyManager) RefreshIPList() error

// 添加到黑名单
func (m *ProxyManager) AddBlacklist(proxyID int)

// 获取黑名单状态
func (m *ProxyManager) GetBlacklistStatus() (total, blacklisted, available int)

// 启动自动刷新
func (m *ProxyManager) StartAutoRefresh()

// 停止自动刷新
func (m *ProxyManager) StopAutoRefresh()
```

## 黑名单机制

### 工作原理

1. **添加黑名单**：当代理请求失败时，调用 `AddBlacklist(proxyID)` 将该IP加入黑名单
2. **TTL倒计时**：每次刷新IP列表时，黑名单中的IP的TTL减1
3. **自动恢复**：当TTL归零时，IP自动从黑名单移除，重新可用

### 线程安全保证

```go
// 添加黑名单使用写锁
func (m *ProxyManager) AddBlacklist(proxyID int) {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    // 防越界检查
    if proxyID < 0 || proxyID >= len(m.ipList) {
        log.Printf("⚠️  无效的 ProxyID: %d", proxyID)
        return
    }

    ip := m.ipList[proxyID].IP
    m.blacklist[proxyID] = ip
    m.ipBlacklist[ip] = m.config.BlacklistTTL
}

// 获取代理使用读锁（支持并发）
func (m *ProxyManager) getRandomProxy() (int, *ProxyIP, error) {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    // ... 读取操作
}
```

### 示例流程

```
初始状态：5个代理IP，TTL=3
IP列表: [IP1, IP2, IP3, IP4, IP5]
黑名单: {}

第1次失败：IP2请求失败
IP列表: [IP1, IP2, IP3, IP4, IP5]
黑名单: {IP2: TTL=3}

第1次刷新：TTL-1
黑名单: {IP2: TTL=2}

第2次刷新：TTL-1
黑名单: {IP2: TTL=1}

第3次刷新：TTL-1
黑名单: {IP2: TTL=0}  → 从黑名单移除

第3次刷新后：
IP列表: [IP1, IP2, IP3, IP4, IP5]
黑名单: {}  ← IP2已恢复可用
```

## 完整使用示例

### 示例1：币安API请求（单代理模式）

```go
package main

import (
    "log"
    "nofx/proxy"
    "time"
)

func main() {
    // 初始化代理
    err := proxy.InitGlobalProxyManager(&proxy.Config{
        Enabled: true,
        Mode: "single",
        ProxyURL: "http://127.0.0.1:7890",
        Timeout: 30 * time.Second,
    })
    if err != nil {
        log.Fatalf("初始化代理失败: %v", err)
    }

    // 获取币安数据
    proxyClient, err := proxy.GetProxyHTTPClient()
    if err != nil {
        log.Fatalf("获取代理客户端失败: %v", err)
    }

    resp, err := proxyClient.Client.Get("https://fapi.binance.com/fapi/v1/ticker/24hr")
    if err != nil {
        log.Printf("请求失败: %v", err)
        return
    }
    defer resp.Body.Close()

    log.Printf("请求成功，使用代理: %s", proxyClient.IP)
}
```

### 示例2：OI数据获取（代理池模式 + 黑名单）

```go
package main

import (
    "fmt"
    "io"
    "log"
    "nofx/proxy"
    "time"
)

func fetchOIData(symbol string) error {
    proxyClient, err := proxy.GetProxyHTTPClient()
    if err != nil {
        return fmt.Errorf("获取代理失败: %w", err)
    }

    url := fmt.Sprintf("https://fapi.binance.com/futures/data/openInterestHist?symbol=%s&period=5m&limit=1", symbol)
    resp, err := proxyClient.Client.Get(url)
    if err != nil {
        // 请求失败，加入黑名单
        proxy.AddBlacklist(proxyClient.ProxyID)
        return fmt.Errorf("请求失败 (代理: %s): %w", proxyClient.IP, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        // 状态码异常，加入黑名单
        proxy.AddBlacklist(proxyClient.ProxyID)
        return fmt.Errorf("状态码异常: %d (代理: %s)", resp.StatusCode, proxyClient.IP)
    }

    body, _ := io.ReadAll(resp.Body)
    log.Printf("✓ 获取 %s OI数据成功 (代理: %s): %s", symbol, proxyClient.IP, string(body))
    return nil
}

func main() {
    // 初始化代理池
    err := proxy.InitGlobalProxyManager(&proxy.Config{
        Enabled: true,
        Mode: "pool",
        ProxyList: []string{
            "http://proxy1.example.com:8080",
            "http://proxy2.example.com:8080",
            "http://proxy3.example.com:8080",
        },
        Timeout: 30 * time.Second,
        BlacklistTTL: 5,
    })
    if err != nil {
        log.Fatalf("初始化代理失败: %v", err)
    }

    // 循环获取数据
    symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
    for {
        for _, symbol := range symbols {
            if err := fetchOIData(symbol); err != nil {
                log.Printf("⚠️  %v", err)
            }
            time.Sleep(1 * time.Second)
        }
        time.Sleep(10 * time.Second)
    }
}
```

### 示例3：Bright Data动态IP

```go
package main

import (
    "log"
    "nofx/proxy"
    "time"
)

func main() {
    // 初始化Bright Data代理
    err := proxy.InitGlobalProxyManager(&proxy.Config{
        Enabled: true,
        Mode: "brightdata",
        BrightDataEndpoint: "https://api.brightdata.com/zones/get_ips",
        BrightDataToken: "your_token",
        BrightDataZone: "residential",
        ProxyHost: "brd.superproxy.io:22225",
        ProxyUser: "brd-customer-xxx-zone-residential-ip-%s",
        ProxyPassword: "your_password",
        RefreshInterval: 30 * time.Minute,
        Timeout: 30 * time.Second,
        BlacklistTTL: 5,
    })
    if err != nil {
        log.Fatalf("初始化代理失败: %v", err)
    }

    // 代理会自动每30分钟刷新IP列表
    log.Println("✓ Bright Data代理已启动，自动刷新已开启")

    // 获取并使用代理
    for i := 0; i < 10; i++ {
        proxyClient, err := proxy.GetProxyHTTPClient()
        if err != nil {
            log.Printf("获取代理失败: %v", err)
            continue
        }

        resp, err := proxyClient.Client.Get("https://api.ipify.org?format=json")
        if err != nil {
            proxy.AddBlacklist(proxyClient.ProxyID)
            log.Printf("请求失败，代理已加入黑名单: %s", proxyClient.IP)
            continue
        }
        resp.Body.Close()

        log.Printf("✓ 请求成功 (代理ID: %d, IP: %s)", proxyClient.ProxyID, proxyClient.IP)
        time.Sleep(2 * time.Second)
    }
}
```

## 注意事项

### 1. 模块解耦性

- ✅ 代理模块完全独立，不依赖其他业务模块
- ✅ 禁用代理时自动使用直连，对业务代码透明
- ✅ 适合多租户/多客户环境，可按需启用

### 2. 线程安全

- ✅ 所有公开方法都是线程安全的
- ✅ 支持高并发场景下的代理获取和黑名单操作
- ✅ 读写锁优化性能：读操作可并发，写操作独占

### 3. 错误处理

```go
proxyClient, err := proxy.GetProxyHTTPClient()
if err != nil {
    // 可能的错误：
    // - 代理IP列表为空
    // - 所有代理都在黑名单中
    // - 代理URL解析失败
    log.Printf("获取代理失败: %v", err)

    // 建议：降级为直连或重试
    return
}
```

### 4. 性能优化建议

- 对于高频请求，复用 `http.Client` 而不是每次创建新的
- 合理设置 `refresh_interval` 避免频繁刷新
- `blacklist_ttl` 建议设置为 3-10，平衡恢复速度和稳定性

### 5. 安全建议

- 生产环境中代理密钥应使用环境变量或密钥管理服务
- 避免在日志中打印完整的代理URL（包含密码）
- TLS验证默认开启，如需跳过请谨慎评估风险

### 6. 调试技巧

```go
// 获取当前代理状态
total, blacklisted, available := proxy.GetGlobalProxyManager().GetBlacklistStatus()
log.Printf("代理池状态: 总计=%d, 黑名单=%d, 可用=%d", total, blacklisted, available)

// 检查是否启用
if !proxy.IsEnabled() {
    log.Println("代理未启用，请检查配置")
}
```

## 故障排查

### 问题1：获取代理失败 - "代理IP列表为空"

**原因**：
- `single` 模式：未配置 `proxy_url`
- `pool` 模式：`proxy_list` 为空
- `brightdata` 模式：API返回空列表或请求失败

**解决方案**：
```bash
# 检查配置文件
cat config.json | grep -A 15 "proxy"

# 检查日志，查看初始化信息
# 应该看到类似：🌐 HTTP 代理已启用 (xxx模式)
```

### 问题2：所有代理都在黑名单中

**原因**：请求持续失败，所有IP被加入黑名单

**解决方案**：
```go
// 方案1：手动刷新IP列表（会触发TTL倒计时）
proxy.RefreshIPList()

// 方案2：降低blacklist_ttl，加快恢复速度
// config.json: "blacklist_ttl": 2  (默认5)

// 方案3：检查代理本身是否可用
// 使用curl测试代理：
// curl -x http://proxy_url https://api.binance.com/api/v3/ping
```

### 问题3：Bright Data模式无法获取IP

**原因**：
- API端点配置错误
- Token无效或过期
- Zone参数不正确

**解决方案**：
```bash
# 手动测试API
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://api.brightdata.com/zones/get_ips?zone=residential"

# 检查返回格式是否符合：
# {"ips": [{"ip": "1.2.3.4", ...}, ...]}
```

### 问题4：代理连接超时

**原因**：代理服务器响应慢或网络不稳定

**解决方案**：
```json
{
  "proxy": {
    "timeout": 60  // 增加超时时间（秒）
  }
}
```

## 扩展开发

### 添加新的Provider

实现 `IPProvider` 接口即可：

```go
// custom_provider.go
package proxy

type CustomProvider struct {
    // 自定义字段
}

func NewCustomProvider(config string) *CustomProvider {
    return &CustomProvider{}
}

func (p *CustomProvider) GetIPList() ([]ProxyIP, error) {
    // 实现获取IP列表的逻辑
    return []ProxyIP{}, nil
}

func (p *CustomProvider) RefreshIPList() ([]ProxyIP, error) {
    // 实现刷新IP列表的逻辑
    return p.GetIPList()
}
```

然后在 `proxy_manager.go` 的 `NewProxyManager` 中添加新模式：

```go
case "custom":
    m.provider = NewCustomProvider(config.CustomEndpoint)
    log.Printf("🌐 HTTP 代理已启用 (自定义模式)")
```

## 更新日志

### v1.0.0 (当前版本)
- ✅ 支持三种代理模式：single、pool、brightdata
- ✅ 线程安全的IP轮换和黑名单管理
- ✅ 自动刷新机制（30分钟默认）
- ✅ TTL黑名单自动恢复
- ✅ 防越界保护
- ✅ ProxyID追踪机制


## 技术支持

如有问题或建议，请联系项目维护者 @hzb1115
。
