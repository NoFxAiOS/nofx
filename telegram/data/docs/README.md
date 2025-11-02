# JT-Bot 数据采集监听器

JT-Bot 是一个专注于 **监听 Telegram 频道并将消息写入 SQLite** 的轻量级工具。当前版本不再包含任何 Telegram Bot 或 Discord 转发逻辑，重点放在稳定采集与持久化存储，方便后续在本地或第三方服务中做二次处理。

## 核心能力

- 📡 监听所有订阅频道或指定白名单频道
- 🧹 消息过滤、去重与文本清洗
- 🗄️ 持久化到 SQLite（默认 `data/jtbot.db`）
- 🧭 运行状态统计（控制台输出）
- 🔐 交互式登录向导（命令 `./start.sh auth`）

## 快速上手

1. **复制配置模板并填写必要参数**
   ```bash
   cp .env.example .env
   # 编辑 .env 补全 TELEGRAM_API_ID、TELEGRAM_API_HASH、TELEGRAM_PHONE_NUMBER 等
   ```

2. **初始化环境**（自动创建虚拟环境并安装依赖）
   ```bash
   ./start.sh setup
   ```

3. **执行 Telegram 登录**（首次运行需要短信或 2FA 验证）
   ```bash
   ./start.sh auth
   ```

4. **启动监听器**
   ```bash
   ./start.sh          # 前台运行并实时输出日志
   ./start.sh --no-follow  # 后台运行
   ```

## 配置说明（`.env`）

| 变量 | 说明 |
| --- | --- |
| `TELEGRAM_API_ID` / `TELEGRAM_API_HASH` | 在 https://my.telegram.org/apps 申请的 API 凭证 |
| `TELEGRAM_PHONE_NUMBER` / `TELEGRAM_PASSWORD` | 登录所用手机号及 2FA 密码（可选） |
| `TELEGRAM_SESSION_NAME` | 生成的 session 文件名前缀，默认 `telegram_monitor_optimized` |
| `LISTEN_ALL_SUBSCRIBED_CHANNELS` | `true` 监听所有已订阅频道；`false` 时仅监听 `CHANNEL_ALLOWLIST` |
| `CHANNEL_ALLOWLIST` | 逗号分隔的频道 ID（带 `-100` 前缀）或用户名 |
| `BLOCK_PRIVATE_MESSAGES` | 是否忽略私聊消息 |
| `BLOCKED_SENDER_IDS` | 以逗号分隔的发送者 ID 黑名单（默认包含官方通知 `777000`） |
| `DATABASE_PATH` | SQLite 数据库存储位置，默认 `./data/jtbot.db` |
| `USE_PROXY` / `PROXY_TYPE` / `PROXY_HOST` / `PROXY_PORT` | Telethon 连接代理设置 |

> 修改 `.env` 后，可执行 `./start.sh restart` 使配置生效。

## 常用命令

```bash
./start.sh setup       # 自检环境并安装依赖
./start.sh auth        # 进入 Telegram 登录向导
./start.sh             # 启动监听器（默认前台跟随日志）
./start.sh --no-follow # 后台运行
./start.sh status      # 查看监听状态与数据库信息
./start.sh stop        # 停止监听进程
./start.sh clean       # 清理日志与 PID 文件
```

日志保存在 `logs/jt_bot.log`；采集的数据默认写入 `data/jtbot.db` 的 `news` 表。

## 代码结构

```
jt-bot/
├── start.sh                    # 统一启动脚本
├── telegram_collector/
│   ├── jt_bot.py               # 监听与入库逻辑
│   ├── settings.py             # 配置加载与路径管理
│   └── config_manager.py       # 工具化路径访问
├── docs/                       # 文档
├── data/                       # 数据与 session
├── logs/                       # 运行日志
├── requirements.txt            # 依赖清单
└── setup.py                    # 包装脚本（可选）
```

## 运行状态与数据库

- 监听过程中可在终端看到状态报告（接收/过滤/写入统计）。
- SQLite 数据库默认结构：
  ```sql
  CREATE TABLE IF NOT EXISTS news (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      title TEXT,
      content TEXT,
      source TEXT,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```
- 可使用 `sqlite3 data/jtbot.db 'SELECT * FROM news ORDER BY created_at DESC LIMIT 10;'` 查看最新数据。

## 注意事项

- Session 文件位于 `telegram_collector/data/sessions/`，请妥善保管。
- 若网络环境受限，可在 `.env` 中配置代理；脚本会自动尝试直连并在失败时回退到代理。
- 本项目默认仅负责采集和存储，任何下游推送或分析需由外部服务自行消费数据库数据。

