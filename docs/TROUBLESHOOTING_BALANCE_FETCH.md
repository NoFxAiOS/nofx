# 获取余额失败问题排查指南

## 问题描述

在配置交易员时,点击"获取当前余额"按钮时出现错误提示。

## 常见错误及解决方案

### 1. "交易员不存在,请刷新页面后重试" (404)

**原因**: trader_id 无效或 trader 已被删除

**解决方案**:
1. 刷新页面 (F5)
2. 重新进入配置页面
3. 如果问题持续,检查 trader 是否在列表中

### 2. "登录已过期,请重新登录" (401)

**原因**: 认证 token 过期

**解决方案**:
1. 退出登录
2. 重新登录
3. 再次尝试获取余额

### 3. "获取余额失败: ... 提示: 请确保交易员已启动并且交易所配置正确" (500)

**最常见的问题!**

**可能的原因**:

#### 原因 A: 交易员未启动
- 交易员处于停止状态
- 无法连接到 exchange API

**解决方案**:
1. 返回 Traders 列表页面
2. 检查 trader 的运行状态
3. 如果显示"已停止",点击"启动"按钮
4. 等待几秒让 trader 初始化
5. 重新打开配置页面并尝试获取余额

#### 原因 B: Exchange 配置问题
- API Key 未配置
- API Key 权限不足
- API Secret 错误
- 网络无法访问 exchange

**解决方案**:
1. 进入 Exchange 配置页面
2. 检查 API Key 和 Secret 是否正确
3. 确认 API Key 有以下权限:
   - 读取账户信息
   - 读取持仓信息
   - 交易权限 (如果需要实盘交易)
4. 测试网络连接到 exchange

#### 原因 C: Paper Trading (模拟交易) 未正确初始化
- 如果使用 paper trading,可能数据库状态异常

**解决方案**:
1. 重启交易员
2. 检查后端日志
3. 如果问题持续,删除 trader 并重新创建

### 4. "只有在编辑模式下才能获取当前余额"

**原因**: 在创建新 trader 时点击了获取余额按钮

**解决方案**:
- 这个功能只在编辑现有 trader 时可用
- 创建新 trader 时需要手动输入初始余额
- 创建后可以编辑 trader 来使用此功能

## 诊断步骤

### 步骤 1: 检查后端服务

```bash
# 检查后端进程
ps aux | grep "./nofx" | grep -v grep

# 检查端口
lsof -ti:8080

# 测试健康检查
curl http://localhost:8080/api/health
```

### 步骤 2: 检查浏览器控制台

1. 打开浏览器开发者工具 (F12)
2. 切换到 Console 标签
3. 点击"获取当前余额"按钮
4. 查看错误信息

**正常情况**应该看到:
```
已获取当前余额: 100.00
```

**错误情况**会看到具体的错误信息,例如:
```
获取余额失败: Error: 获取余额失败: ...
```

### 步骤 3: 检查 Network 请求

1. 在开发者工具中切换到 Network 标签
2. 点击"获取当前余额"按钮
3. 找到 `/api/account?trader_id=...` 请求
4. 检查:
   - Status Code (应该是 200)
   - Request Headers (Authorization 应该存在)
   - Response (查看错误详情)

### 步骤 4: 检查 Trader 状态

```bash
# 进入项目目录
cd /Users/xyh/Code/nofx

# 查询 trader 列表
sqlite3 config.db "SELECT id, name, is_running FROM traders;"

# 检查 exchange 配置
sqlite3 config.db "SELECT id, enabled, api_key FROM exchanges;"
```

### 步骤 5: 查看后端日志

如果后端有日志输出,查找类似信息:
```
📊 收到账户信息请求 [TraderName]
❌ 获取账户信息失败 [TraderName]: ...
```

## 手动测试 API

使用 curl 测试完整的 API 调用:

```bash
# 1. 获取你的 auth_token (从浏览器控制台)
TOKEN="your_auth_token_here"

# 2. 获取你的 trader_id (从浏览器控制台或数据库)
TRADER_ID="your_trader_id_here"

# 3. 测试 API
curl -H "Authorization: Bearer $TOKEN" \
     "http://localhost:8080/api/account?trader_id=$TRADER_ID"
```

**成功响应示例**:
```json
{
  "total_equity": 100.50,
  "available_balance": 95.00,
  "total_pnl": 0.50,
  "total_pnl_pct": 0.50,
  "position_count": 1,
  "margin_used_pct": 5.0
}
```

**错误响应示例**:
```json
{
  "error": "获取账户信息失败: ..."
}
```

## 最佳实践

### 使用"获取当前余额"功能前:

1. ✅ 确保 trader 已启动
2. ✅ 确认 exchange 配置正确
3. ✅ 等待几秒让 trader 初始化
4. ✅ 在编辑模式下使用此功能

### Paper Trading 用户:

- Paper trading 会模拟账户余额
- 第一次启动时,余额基于配置的 initial_balance
- 可以随时使用"获取当前余额"来查看当前模拟账户状态

### Real Trading 用户:

- 确保 API Key 有足够权限
- 检查 IP 白名单设置
- 确认网络可以访问 exchange API
- 首次使用建议先用 paper trading 测试

## 联系支持

如果以上方法都无法解决问题,请提供以下信息:

1. 浏览器控制台的完整错误信息
2. Network 标签中 `/api/account` 请求的详细信息
3. Trader 配置 (隐藏敏感信息)
4. Exchange 类型 (Binance/Hyperliquid/Paper Trading)
5. 是否首次使用此功能

## 相关文档

- [Exchange 配置指南](./EXCHANGE_CONFIGURATION.md)
- [Paper Trading 文档](../PAPER_TRADING_FIXED.md)
- [API 文档](./API.md)
