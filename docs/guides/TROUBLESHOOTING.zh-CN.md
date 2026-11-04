# 🔧 故障排查指南

本指南帮助您在提交 bug 报告前自行诊断和修复常见问题。

---

## 📋 快速诊断清单

提交 bug 前，请检查：

1. ✅ **后端正在运行**: `docker compose ps` 或 `ps aux | grep nofx`
2. ✅ **前端可访问**: 在浏览器打开 http://localhost:3000
3. ✅ **API 正常响应**: `curl http://localhost:8080/api/health`
4. ✅ **检查日志中的错误**: 参见下方 [如何捕获日志](#如何捕获日志)

---

## 🐛 常见问题与解决方案

### 1. 交易问题

#### ❌ 只开空单，不开多单 (Issue #202)

**症状:** AI 只开空仓，从不开多仓，即使市场看涨。

**根本原因:** 币安账户处于**单向持仓模式**而非**双向持仓模式**。

**解决方案:**
1. 登录 [币安合约交易](https://www.binance.com/zh-CN/futures/BTCUSDT)
2. 点击右上角 **⚙️ 偏好设置**
3. 选择 **持仓模式**
4. 切换为 **双向持仓** (Hedge Mode)
5. ⚠️ **重要:** 切换前必须先平掉所有持仓

**为什么会这样:**
- 代码使用 `PositionSide(LONG)` 和 `PositionSide(SHORT)` 参数
- 这些参数只在双向持仓模式下有效
- 在单向持仓模式下，订单会失败或只有一个方向有效

**关于子账户:**
- 部分币安子账户可能没有权限更改持仓模式
- 使用主账户或联系币安客服开通此权限

---

#### ❌ 订单错误: `code=-4061` 持仓方向不匹配

**错误信息:** `Order's position side does not match user's setting`

**解决方案:** 同上 - 切换到双向持仓模式。

---

#### ❌ 杠杆错误: `子账户限制最高5倍杠杆`

**症状:** 尝试使用 >5倍杠杆时订单失败。

**解决方案:**
1. 打开 Web 界面 → 交易员设置
2. 将杠杆设置为 5倍或更低:
   ```json
   {
     "btc_eth_leverage": 5,
     "altcoin_leverage": 5
   }
   ```
3. 或使用主账户（支持最高 50倍 BTC/ETH，20倍山寨币）

---

#### ❌ 持仓无法执行

**检查以下内容:**
1. **API 权限**:
   - 进入币安 → API 管理
   - 确认"启用合约"已勾选
   - 检查 IP 白名单（如果启用）

2. **账户余额**:
   - 确保合约钱包中有足够的 USDT
   - 检查保证金使用率未达到 100%

3. **交易对状态**:
   - 确认交易对在交易所处于活跃状态
   - 检查交易对是否处于维护模式

4. **决策日志**:
   ```bash
   # 检查最新决策
   ls -lt decision_logs/your_trader_id/ | head -5
   cat decision_logs/your_trader_id/latest_file.json
   ```
   - 查看 AI 决策：是"wait"、"hold"还是实际交易？
   - 检查 position_size_usd 是否在限制范围内

---

### 2. AI 决策问题

#### ❌ AI 总是说"等待"/"持有"

**可能原因:**
1. **市场情况**: AI 可能确实没看到好的机会
2. **风险限制**: 账户净值太低、保证金使用率太高
3. **历史表现**: AI 在亏损后变得谨慎

**如何检查:**
```bash
# 查看最新决策推理
cat decision_logs/your_trader_id/$(ls -t decision_logs/your_trader_id/ | head -1)
```

查看 AI 的思维链（Chain-of-Thought）推理部分。

**解决方案:**
- 等待更好的市场条件
- 检查候选币种是否流动性都太低
- 确认 `use_default_coins: true` 或币种池 API 正常工作

---

#### ❌ AI 做出错误决策

**请记住:** AI 交易是实验性的，不保证盈利。

**需要检查的事项:**
1. **决策间隔**: 是否太短？（推荐: 3-5分钟）
2. **杠杆设置**: 是否过于激进？
3. **历史反馈**: 查看表现日志，看 AI 是否在学习
4. **市场波动**: 高波动 = 更高风险

**调整建议:**
- 降低杠杆以实现更保守的交易
- 增加决策间隔以减少过度交易
- 使用较小的初始余额进行测试

---

### 3. 连接和 API 问题

#### ❌ Docker 镜像下载失败 (中国大陆)

**错误:** `ERROR [internal] load metadata for docker.io/library/...`

**症状:**
- `docker compose build` 或 `docker compose up` 卡住
- 超时错误: `timeout`、`connection refused`
- 无法从 Docker Hub 拉取镜像

**根本原因:**
中国大陆访问 Docker Hub 受限或速度极慢。

**解决方案 1: 配置 Docker 镜像加速器（推荐）**

1. **编辑 Docker 配置文件:**
   ```bash
   # Linux
   sudo nano /etc/docker/daemon.json

   # macOS (Docker Desktop)
   # Settings → Docker Engine
   ```

2. **添加国内镜像源:**
   ```json
   {
     "registry-mirrors": [
       "https://docker.m.daocloud.io",
       "https://docker.1panel.live",
       "https://hub.rat.dev",
       "https://dockerpull.com",
       "https://dockerhub.icu"
     ]
   }
   ```

3. **重启 Docker:**
   ```bash
   # Linux
   sudo systemctl restart docker

   # macOS/Windows
   # 重启 Docker Desktop
   ```

4. **重新构建:**
   ```bash
   docker compose build --no-cache
   docker compose up -d
   ```

**解决方案 2: 使用 VPN**

1. 连接 VPN（推荐台湾节点）
2. 确保使用**全局模式**而非规则模式
3. 重新运行 `docker compose build`

**解决方案 3: 离线下载镜像**

如果上述方法都不行:

1. **使用镜像代理网站下载:**
   - https://proxy.vvvv.ee/images.html （可离线下载）
   - https://github.com/dongyubin/DockerHub （镜像加速列表）

2. **手动导入镜像:**
   ```bash
   # 下载镜像文件后
   docker load -i golang-1.25-alpine.tar
   docker load -i node-20-alpine.tar
   docker load -i nginx-alpine.tar
   ```

3. **验证镜像已加载:**
   ```bash
   docker images | grep golang
   docker images | grep node
   docker images | grep nginx
   ```

**验证镜像加速器是否生效:**
```bash
# 查看 Docker 信息
docker info | grep -A 10 "Registry Mirrors"

# 应该显示你配置的镜像源
```

**相关 Issue:** [#168](https://github.com/NoFxAiOS/nofx/issues/168)

---

#### ❌ 后端无法启动

**错误:** `port 8080 already in use`

**解决方案:**
```bash
# 查找占用端口的进程
lsof -i :8080
# 或
netstat -tulpn | grep 8080

# 杀死进程或在 .env 中更改端口
NOFX_BACKEND_PORT=8081
```

---

#### ❌ 前端无法连接后端

**症状:**
- UI 显示"加载中..."一直不结束
- 浏览器控制台显示 404 或网络错误

**解决方案:**
1. **检查后端是否运行:**
   ```bash
   docker compose ps  # 应显示 backend 为 "Up"
   # 或
   curl http://localhost:8080/api/health  # 应返回 {"status":"ok"}
   ```

2. **检查端口配置:**
   - 后端默认: 8080
   - 前端默认: 3000
   - 确认 `.env` 设置匹配

3. **CORS 问题:**
   - 如果前端和后端运行在不同端口/域名
   - 检查浏览器控制台的 CORS 错误
   - 后端应允许前端来源

---

#### ❌ 交易所 API 错误

**常见错误:**
- `code=-1021, msg=Timestamp for this request is outside of the recvWindow`
- `invalid signature`
- `timestamp` 错误

**根本原因:**
系统时间不准确，与币安服务器时间相差超过允许范围（通常是 5 秒）。

**解决方案 1: 同步系统时间（推荐）**

```bash
# 方法 1: 使用 ntpdate (最常用)
sudo ntpdate pool.ntp.org

# 方法 2: 使用其他 NTP 服务器
sudo ntpdate -s time.nist.gov
sudo ntpdate -s ntp.aliyun.com  # 阿里云 NTP (中国大陆快)

# 方法 3: 启用自动时间同步 (Linux)
sudo timedatectl set-ntp true

# 验证时间是否正确
date
# 应该显示正确的当前时间
```

**Docker 环境特别注意:**

如果使用 Docker，容器时间可能与宿主机不同步：

```bash
# 检查容器时间
docker exec nofx-backend date

# 如果时间错误，重启 Docker 服务
sudo systemctl restart docker

# 或在 docker-compose.yml 中添加时区设置
environment:
  - TZ=Asia/Shanghai  # 或您的时区
```

**解决方案 2: 验证 API 密钥**

如果时间同步后仍有错误：

1. **检查 API 密钥:**
   - 未过期
   - 有正确权限（已启用合约）
   - IP 白名单包含您的服务器 IP

2. **重新生成 API 密钥:**
   - 登录币安 → API 管理
   - 删除旧密钥
   - 创建新密钥
   - 更新 NOFX 配置

**解决方案 3: 检查速率限制**

币安有严格的 API 速率限制：

- **每分钟请求数限制**
- 减少交易员数量
- 增加决策间隔时间（例如从 1 分钟改为 3-5 分钟）

**相关 Issue:** [#60](https://github.com/NoFxAiOS/nofx/issues/60)

---

### 4. 前端问题

#### ❌ UI 不更新 / 显示旧数据

**解决方案:**
1. **强制刷新:**
   - Chrome/Firefox: `Ctrl+Shift+R` (Windows/Linux) 或 `Cmd+Shift+R` (Mac)
   - Safari: `Cmd+Option+R`

2. **清除浏览器缓存:**
   - 设置 → 隐私 → 清除浏览数据
   - 或在无痕/隐私模式下打开

3. **检查 SWR 轮询:**
   - 前端使用 5-10秒间隔的 SWR
   - 数据应自动刷新
   - 检查浏览器控制台是否有 fetch 错误

---

#### ❌ 图表不渲染

**可能原因:**
1. 暂无历史数据（系统刚启动）
2. 控制台中有 JavaScript 错误
3. 浏览器兼容性问题

**解决方案:**
- 等待 5-10 分钟让数据积累
- 检查浏览器控制台（F12）是否有错误
- 尝试不同浏览器（推荐 Chrome）
- 确保后端 API 端点正在返回数据

---

### 5. 数据库问题

#### ❌ `database is locked` 错误

**原因:** SQLite 数据库被多个进程访问。

**解决方案:**
```bash
# 停止所有 NOFX 进程
docker compose down
# 或
pkill nofx

# 重启
docker compose up -d
# 或
./nofx
```

---

#### ❌ 交易员配置无法保存

**检查:**
1. **PostgreSQL 容器状态**
   ```bash
   docker compose ps postgres
   docker compose exec postgres pg_isready -U nofx -d nofx
   ```

2. **直接检查数据库数据**
   ```bash
   ./scripts/view_pg_data.sh                        # 快速总览
   docker compose exec postgres \
     psql -U nofx -d nofx -c "SELECT COUNT(*) FROM traders;"
   ```

3. **磁盘空间**
   ```bash
   df -h  # 确保磁盘未满
   ```

---

## 📊 如何捕获日志

### 后端日志

**Docker:**
```bash
# 查看最后 100 行
docker compose logs backend --tail=100

# 实时跟踪日志
docker compose logs -f backend

# 保存到文件
docker compose logs backend --tail=500 > backend_logs.txt
```

**手动运行:**
```bash
# 如果不是通过 Docker，而是手动运行 ./nofx，可直接在终端查看日志
```

---

### 前端日志（浏览器控制台）

1. **打开开发者工具:**
   - 按 `F12` 或右键 → 检查

2. **Console（控制台）标签:**
   - 查看 JavaScript 错误和警告
   - 寻找红色错误消息

3. **Network（网络）标签:**
   - 按"XHR"或"Fetch"筛选
   - 查找失败的请求（红色状态码）
   - 点击失败的请求 → Preview/Response 查看错误详情

4. **捕获截图:**
   - Windows: `Win+Shift+S`
   - Mac: `Cmd+Shift+4`
   - 或使用浏览器开发者工具截图功能

---

### 决策日志（交易问题）

```bash
# 列出最近的决策日志
ls -lt decision_logs/your_trader_id/ | head -10

# 查看最新决策
cat decision_logs/your_trader_id/$(ls -t decision_logs/your_trader_id/ | head -1) | jq .

# 搜索特定交易对
grep -r "BTCUSDT" decision_logs/your_trader_id/

# 查找执行交易的决策
grep -r '"action": "open_' decision_logs/your_trader_id/
```

**决策日志中要查看的内容:**
- `chain_of_thought`: AI 的推理过程
- `user_prompt`: AI 收到的市场数据
- `decision`: 最终决策（动作、交易对、杠杆等）
- `account_state`: 决策时的账户余额、保证金、持仓
- `execution_result`: 交易是否成功

---

## 🔍 诊断命令

### 系统健康检查

```bash
# 后端健康状态
curl http://localhost:8080/api/health

# 列出所有交易员
curl http://localhost:8080/api/traders

# 检查特定交易员状态
curl http://localhost:8080/api/status?trader_id=your_trader_id

# 获取账户信息
curl http://localhost:8080/api/account?trader_id=your_trader_id
```

### Docker 状态

```bash
# 检查所有容器
docker compose ps

# 检查资源使用
docker stats

# 重启特定服务
docker compose restart backend
docker compose restart frontend
```

### 数据库查询

```bash
# 检查数据库中的交易员
docker compose exec postgres \
  psql -U nofx -d nofx -c "SELECT id, name, ai_model_id, exchange_id, is_running FROM traders;"

# 检查 AI 模型
docker compose exec postgres \
  psql -U nofx -d nofx -c "SELECT id, name, provider, enabled FROM ai_models;"

# 检查系统配置
docker compose exec postgres \
  psql -U nofx -d nofx -c "SELECT key, value FROM system_config;"
```

---

## 📝 仍有问题？

如果尝试了上述所有方法仍有问题:

1. **收集信息:**
   - 后端日志（最后 100 行）
   - 前端控制台截图
   - 决策日志（如果是交易问题）
   - 您的环境详情

2. **提交 Bug 报告:**
   - 使用 [Bug 报告模板](../../.github/ISSUE_TEMPLATE/bug_report.md)
   - 包含所有日志和截图
   - 描述您已尝试的方法

3. **加入社区:**

   - [GitHub Discussions](https://github.com/NoFxAiOS/nofx/discussions)

---

## 🆘 紧急情况：系统完全损坏

**完全重置 (⚠️ 将丢失交易历史):**

```bash
# 停止所有服务
docker compose down

# 可选：备份 PostgreSQL 数据
docker compose exec postgres \
  pg_dump -U nofx -d nofx > backup_nofx.sql

# 删除所有持久化卷（全新开始）
docker compose down -v

# 重启
docker compose up -d --build

# 通过 Web UI 重新配置
open http://localhost:3000
```

**部分重置（保留配置，清除日志）:**

```bash
# 清除决策日志
rm -rf decision_logs/*

# 清除 Docker 缓存并重建
docker compose down
docker compose build --no-cache
docker compose up -d
```

---

## 📚 其他资源

- **[FAQ](faq.zh-CN.md)** - 常见问题
- **[快速开始](../getting-started/README.zh-CN.md)** - 安装指南
- **[架构文档](../architecture/README.zh-CN.md)** - 系统工作原理
- **[CLAUDE.md](../../CLAUDE.md)** - 开发者文档

---

**最后更新:** 2025-11-02
