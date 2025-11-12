# Hook 模块使用文档

## 简介

Hook模块提供了一个通用的扩展点机制，允许在不修改核心代码的前提下注入自定义逻辑。

**核心特点**：
- 类型安全的泛型API
- Hook未注册时自动fallback
- 支持任意参数和返回值

## 快速开始

### 基本用法

```go
// 1. 注册Hook
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    userId := args[0].(string)
    return &hook.IpResult{IP: "192.168.1.1"}
})

// 2. 调用Hook
result := hook.HookExec[hook.IpResult](hook.GETIP, "user123")
if result != nil && result.Error() == nil {
    ip := result.GetResult()
}
```

### 核心API

```go
// 注册Hook函数
func RegisterHook(key string, hook HookFunc)

// 执行Hook（泛型）
func HookExec[T any](key string, args ...any) *T
```

## 可用的Hook扩展点

### 1. `GETIP` - 获取用户IP

**调用位置**：`api/server.go:210`

**参数**：`userId string`

**返回**：`*IpResult`
```go
type IpResult struct {
    Err error
    IP  string
}
```

**用途**：返回用户专用IP（如代理IP）

---

### 2. `NEW_BINANCE_TRADER` - Binance客户端创建

**调用位置**：`trader/binance_futures.go:68`

**参数**：`userId string, client *futures.Client`

**返回**：`*NewBinanceTraderResult`
```go
type NewBinanceTraderResult struct {
    Err    error
    Client *futures.Client  // 可修改client配置
}
```

**用途**：为Binance客户端注入代理、日志等

---

### 3. `NEW_ASTER_TRADER` - Aster客户端创建

**调用位置**：`trader/aster_trader.go:68`

**参数**：`user string, client *http.Client`

**返回**：`*NewAsterTraderResult`
```go
type NewAsterTraderResult struct {
    Err    error
    Client *http.Client  // 可修改HTTP client
}
```

**用途**：为Aster客户端注入代理等

## 使用示例

### 示例1：代理模块注册Hook

```go
// proxy/init.go
package proxy

import "nofx/hook"

func InitHooks(enabled bool) {
    if !enabled {
        return  // 条件不满足，不注册
    }

    // 注册IP获取Hook
    hook.RegisterHook(hook.GETIP, func(args ...any) any {
        userId := args[0].(string)
        proxyIP, err := getProxyIP(userId)
        return &hook.IpResult{Err: err, IP: proxyIP}
    })

    // 注册Binance客户端Hook
    hook.RegisterHook(hook.NEW_BINANCE_TRADER, func(args ...any) any {
        userId := args[0].(string)
        client := args[1].(*futures.Client)

        // 修改client配置
        if client.HTTPClient != nil {
            client.HTTPClient.Transport = getProxyTransport()
        }

        return &hook.NewBinanceTraderResult{Client: client}
    })
}
```

## 最佳实践

### ✅ 推荐做法

```go
// 1. 在注册时判断条件
func InitHooks(enabled bool) {
    if !enabled {
        return  // 不注册
    }
    hook.RegisterHook(KEY, hookFunc)
}

// 2. 总是返回正确的Result类型
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    ip, err := getIP()
    return &hook.IpResult{Err: err, IP: ip}  // ✅
})

// 3. 安全的类型断言
userId, ok := args[0].(string)
if !ok {
    return &hook.IpResult{Err: fmt.Errorf("参数类型错误")}
}
```

### ❌ 避免的做法

```go
// 1. 不要在Hook内部判断条件（浪费性能）
hook.RegisterHook(KEY, func(args ...any) any {
    if !enabled {
        return nil  // ❌
    }
    // ...
})

// 2. 不要直接panic
hook.RegisterHook(KEY, func(args ...any) any {
    if err != nil {
        panic(err)  // ❌ 会导致程序崩溃
    }
})

// 3. 不要跳过类型检查
userId := args[0].(string)  // ❌ 可能panic
```

## 添加新Hook扩展点

### 步骤1：定义Result类型

```go
// hook/my_hook.go
package hook

type MyHookResult struct {
    Err    error
    Data   string
}

func (r *MyHookResult) Error() error {
    if r.Err != nil {
        log.Printf("⚠️ Hook出错: %v", r.Err)
    }
    return r.Err
}

func (r *MyHookResult) GetResult() string {
    r.Error()
    return r.Data
}
```

### 步骤2：定义Hook常量

```go
// hook/hooks.go
const (
    GETIP              = "GETIP"
    NEW_BINANCE_TRADER = "NEW_BINANCE_TRADER"
    NEW_ASTER_TRADER   = "NEW_ASTER_TRADER"
    MY_HOOK            = "MY_HOOK"  // 新增
)
```

### 步骤3：在业务代码调用

```go
result := hook.HookExec[hook.MyHookResult](hook.MY_HOOK, arg1, arg2)
if result != nil && result.Error() == nil {
    data := result.GetResult()
    // 使用data
}
```

### 步骤4：注册实现

```go
hook.RegisterHook(hook.MY_HOOK, func(args ...any) any {
    // 处理逻辑
    return &hook.MyHookResult{Data: "result"}
})
```

## 常见问题

**Q: Hook可以注册多个吗？**
A: 不可以，每个Key只能注册一个Hook，后注册会覆盖前面的。如需多个逻辑，请在一个Hook函数内组合。

**Q: Hook执行失败会影响主流程吗？**
A: 不会，主流程会检查返回值，失败时会fallback到默认逻辑。

**Q: 如何调试Hook？**
A: Hook执行时会自动打印日志：
- `🔌 Execute hook: {KEY}` - Hook存在并执行
- `🔌 Do not find hook: {KEY}` - Hook未注册

**Q: 如何测试Hook？**
```go
func TestHook(t *testing.T) {
    // 清空全局Hook
    hook.Hooks = make(map[string]hook.HookFunc)

    // 注册测试Hook
    hook.RegisterHook(hook.GETIP, func(args ...any) any {
        return &hook.IpResult{IP: "127.0.0.1"}
    })

    // 验证
    result := hook.HookExec[hook.IpResult](hook.GETIP, "test")
    assert.Equal(t, "127.0.0.1", result.IP)
}
```

## 参考

- 核心实现：`hook/hooks.go`
- Result类型：`hook/trader_hook.go`, `hook/ip_hook.go`
- 调用示例：`api/server.go`, `trader/binance_futures.go`, `trader/aster_trader.go`
