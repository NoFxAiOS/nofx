# 📘 Binance Testnet 配置指南

## 🎯 概述

NOFX 使用 Binance Testnet 进行模拟交易测试。本指南将帮助你正确配置 Binance Testnet 账户。

---

## 🚀 快速开始

### 1️⃣ 激活 Testnet 账户

1. **访问 Testnet 网站**
   ```
   https://testnet.binance.vision/
   ```

2. **使用 GitHub 账号登录**
   - 点击右上角 "Login with GitHub"
   - 授权 Binance Testnet 应用

3. **生成 API 密钥**
   - 登录后点击 "Generate HMAC_SHA256 Key"
   - 系统会自动：
     - ✅ 激活你的账户
     - ✅ 发放 **10,000 USDT** 测试资金
     - ✅ 生成 API Key 和 Secret Key

4. **保存 API 密钥**
   ```
   API Key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
   Secret Key: yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy
   ```
   ⚠️ **重要**: Secret Key 只显示一次，请妥善保存！

---

### 2️⃣ 在 NOFX 中配置

1. **启动 NOFX 系统**
   ```bash
   cd /Users/xyh/Code/nofx
   go run main.go
   ```

2. **访问 Web 界面**
   ```
   http://localhost:3002
   ```

3. **配置交易所**
   - 进入 "Settings" → "Exchanges"
   - 选择 "Binance Futures (Testnet)"
   - 填入你的 API Key 和 Secret Key
   - 点击 "Save"

---

## 🔍 常见问题

### ❌ 问题 1: "This account is inactive"

**错误信息：**
```
code=-4109, msg=This account is inactive, please activate the account first.
```

**解决方案：**
1. 访问 https://testnet.binance.vision/
2. 登录并点击 "Generate HMAC_SHA256 Key"
3. 账户会自动激活

---

### ❌ 问题 2: WebSocket 连接失败

**错误信息：**
```
❌ 批量订阅流失败: dial tcp: lookup fstream.binance.com: no such host
```

**原因：**
- 网络连接问题
- DNS 解析失败
- 需要代理访问

**解决方案：**
1. **检查网络连接**
   ```bash
   ping fstream.binance.com
   ```

2. **使用备用 DNS**
   ```bash
   # 修改 /etc/hosts (macOS/Linux)
   sudo echo "54.194.168.31 fstream.binance.com" >> /etc/hosts
   ```

3. **配置代理** (如果在中国大陆)
   ```bash
   export https_proxy=http://127.0.0.1:7890
   export http_proxy=http://127.0.0.1:7890
   ```

**系统行为：**
- ✅ WebSocket 连接失败不会影响系统运行
- ✅ 系统会使用历史数据继续工作
- ✅ AI 决策功能正常
- ⚠️ 实时价格更新会延迟

---

### ❌ 问题 3: API 调用失败

**错误信息：**
```
❌ 币安API调用失败: <APIError> rsp=
```

**原因：**
- API Key 配置错误
- API Key 权限不足
- 服务器时间不同步

**解决方案：**
1. **验证 API Key**
   - 确保 API Key 和 Secret Key 正确
   - 重新生成 API Key

2. **同步系统时间**
   ```bash
   # macOS
   sudo sntp -sS time.apple.com
   
   # Linux
   sudo ntpdate -s time.nist.gov
   ```

3. **测试 API 连接**
   ```bash
   curl -X GET "https://testnet.binance.vision/fapi/v1/time"
   ```

---

## 📊 验证配置

### ✅ 成功标志

启动系统后，你应该看到：

```
✅ 历史数据加载成功
2025/11/11 12:33:00 已加载 BTCUSDT 的历史K线数据-3m: 100 条
2025/11/11 12:33:00 已加载 ETHUSDT 的历史K线数据-3m: 100 条

✅ 服务器时间同步成功
2025/11/11 12:39:27 ⏱ 已同步币安服务器时间，偏移 -250ms

✅ 账户配置正确
✓ 账户已是双向持仓模式（Hedge Mode）

✅ 账户信息获取成功
余额: 10000.00 USDT
```

---

## 🌐 Testnet 限制

### 功能限制
- ⚠️ 仅支持期货合约交易
- ⚠️ 不支持现货交易
- ⚠️ 市场深度可能较浅
- ⚠️ 订单簿可能不活跃

### 资金限制
- 💰 初始资金: 10,000 USDT
- 🔄 可重置: 访问 Testnet 网站重置账户

### 时间限制
- 🕐 API Key 永久有效
- 🔑 可随时重新生成

---

## 🔧 高级配置

### 代理设置

如果需要通过代理访问 Binance：

```bash
# 设置环境变量
export HTTPS_PROXY=http://127.0.0.1:7890
export HTTP_PROXY=http://127.0.0.1:7890

# 启动系统
go run main.go
```

### 自定义端点

编辑 `market/combined_streams.go`：

```go
endpoints := []string{
    "wss://fstream.binance.com/stream",
    "wss://stream.binance.com:9443/stream",
    "wss://your-proxy-endpoint/stream", // 添加你的代理端点
}
```

---

## 📞 获取帮助

- 📖 [Binance Testnet 官方文档](https://testnet.binance.vision/)
- 📖 [Binance API 文档](https://binance-docs.github.io/apidocs/futures/cn/)
- 💬 [NOFX GitHub Issues](https://github.com/XiaYiHann/nofx/issues)

---

## 📝 检查清单

配置完成后，请确认：

- [ ] ✅ Testnet 账户已激活
- [ ] ✅ API Key 已生成并保存
- [ ] ✅ API Key 已在 NOFX 中配置
- [ ] ✅ 历史数据加载成功
- [ ] ✅ 服务器时间已同步
- [ ] ✅ 账户信息获取成功
- [ ] ✅ 双向持仓模式已启用

完成所有步骤后，你就可以开始使用 NOFX 进行模拟交易了！🎉
