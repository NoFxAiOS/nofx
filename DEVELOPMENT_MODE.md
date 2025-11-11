# 开发模式指南 (Development Mode Guide)

## 🚀 开发模式概述

NOFX系统支持开发模式，在该模式下可以禁用2FA（双因素认证）以简化开发和测试流程。

## 🔧 禁用2FA的方法

### 方法一：使用 `./start.sh start --dev`（推荐）

这是最简单的方法，脚本会自动设置开发环境变量：

```bash
# 启动开发模式（自动禁用2FA）
./start.sh start --dev
```

**特点：**
- ✅ 自动禁用2FA验证
- ✅ 启动前端开发服务器（热重载）
- ✅ 启动后端服务
- ✅ 无需手动配置环境变量

### 方法二：手动设置环境变量

如果您想更灵活地控制，可以手动设置环境变量：

1. **创建 `.env` 文件**：
```bash
cp .env.example .env
```

2. **编辑 `.env` 文件**：
```env
# 禁用2FA（开发模式）
DISABLE_OTP=true

# 其他配置...
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000
```

3. **启动服务**：
```bash
# 使用现有脚本启动（会读取.env文件）
./start.sh start

# 或直接运行Go程序
DISABLE_OTP=true go run main.go
```

### 方法三：临时环境变量

```bash
# Linux/macOS
DISABLE_OTP=true ./start.sh start

# Windows
set DISABLE_OTP=true
./start.sh start
```

## 📋 开发模式 vs 生产模式对比

| 功能 | 开发模式 (`--dev`) | 生产模式 |
|------|-------------------|----------|
| 2FA验证 | ❌ **已禁用** | ✅ **强制启用** |
| 前端构建 | 🔧 开发服务器 (Vite) | 📦 静态文件 (生产构建) |
| 热重载 | ✅ 支持 | ❌ 不支持 |
| 前端端口 | 3000 | 80 |
| 错误调试 | 🔍 详细日志 | 📋 精简日志 |

## 🛡️ 安全注意事项

### ⚠️ 重要安全警告

1. **仅限开发环境使用**
   - 开发模式禁用的2FA仅适用于本地开发
   - **绝不能在生产环境中使用**

2. **环境变量管理**
   - 不要将包含 `DISABLE_OTP=true` 的 `.env` 文件提交到Git
   - 生产部署前确保移除或重命名 `.env` 文件

3. **代码审查**
   - 确保生产部署代码中没有硬编码的 `DISABLE_OTP=true`

## 🔄 切换回生产模式

### 方法一：使用生产模式启动
```bash
# 停止开发模式服务
./start.sh stop

# 启动生产模式（启用2FA）
./start.sh start
```

### 方法二：修改环境变量
```bash
# 编辑.env文件
sed -i 's/DISABLE_OTP=true/DISABLE_OTP=false/' .env

# 重启服务
./start.sh stop
./start.sh start
```

### 方法三：删除环境变量
```bash
# 删除.env文件中的DISABLE_OTP行
sed -i '/DISABLE_OTP/d' .env

# 或删除整个.env文件
rm .env
```

## 🔍 验证2FA状态

### 检查服务日志
```bash
# 查看后端日志
tail -f nofx.log
```

**开发模式的日志输出：**
```
🚫 OTP已禁用 (开发模式)
```

### 前端测试
1. **注册测试**：
   - 开发模式：直接注册成功，无需OTP
   - 生产模式：需要扫描二维码并输入验证码

2. **登录测试**：
   - 开发模式：输入邮箱密码即可登录
   - 生产模式：需要输入OTP验证码

## 🐛 常见问题

### Q: 开发模式启动后仍然要求OTP验证？
**A:** 检查以下几点：
1. 确认使用了 `./start.sh start --dev` 命令
2. 检查后端日志是否显示 "🚫 OTP已禁用 (开发模式)"
3. 确认没有其他地方设置了环境变量

### Q: 如何在开发模式下测试2FA功能？
**A:** 临时启用2FA：
```bash
# 临时启用2FA
DISABLE_OTP=false ./start.sh start
```

### Q: Docker容器中如何设置？
**A:** 在docker-compose.yml中添加环境变量：
```yaml
services:
  nofx:
    environment:
      - DISABLE_OTP=true  # 仅限开发环境
```

## 📚 相关文档

- [2FA功能恢复说明](2FA_RESTORATION.md)
- [Paper Trading功能说明](PAPER_TRADING_FIXED.md)
- [安全配置指南](SECURITY.md)

---

**💡 提示：** 开发完成后，请务必切换回生产模式以确保系统安全性！