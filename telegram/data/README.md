# JT-Bot - Telegram 消息采集与入库系统

JT-Bot 会自动监听指定的 Telegram 频道或群组，将符合规则的内容清洗后写入 SQLite 数据库。项目提供一键启动脚本，首次部署即可完成环境配置、依赖安装、登录校验与服务启动。

## 快速开始

1. **复制并编辑配置**（如不存在会自动生成）：
   ```bash
   cp .env.example .env
   # 填写 Telegram 相关字段、代理与数据库位置
   ```
2. **初始化环境与依赖**：
   ```bash
   ./start.sh setup
   ```
3. **Telegram 首次认证**（生成会话文件）：
   ```bash
   ./start.sh auth
   ```
4. **启动服务**：
   ```bash
   ./start.sh         # 默认前台跟随日志
   ./start.sh --no-follow  # 后台运行
   ```

## 安装

### 一次性环境初始化

`./start.sh setup` 会自动：

- 检查是否已安装 `python3` 与 `venv`；
- 创建 `.venv` 虚拟环境；
- 安装 `requirements.txt` 中的依赖；
- 在缺失时从 `.env.example` 复制 `.env`。

### 手动方式（备选）

```bash
python3 -m venv .venv
source .venv/bin/activate    # Windows 使用 .venv\Scripts\Activate.ps1
pip install -r requirements.txt
```

## 使用

### 统一启动脚本

```bash
./start.sh auth           # 运行交互式 Telegram 登录向导
./start.sh setup          # 仅执行环境检查（可重复执行）
./start.sh                # 启动监听器并实时打印日志
./start.sh --no-follow    # 启动后返回终端，服务后台运行
./start.sh status         # 查看运行状态与日志路径
./start.sh stop           # 停止所有服务
./start.sh clean          # 清理日志与 PID 文件
./start.sh restart        # 重启（stop + start）
```

### 直接运行（高级用法）

```bash
python -m telegram_collector.jt_bot monitor
```

## 功能特性

- ✅ 异步监听 Telegram 频道/群组
- ✅ 灵活的消息过滤、正则清洗与格式化
- ✅ SQLite 持久化，避免消息丢失
- ✅ CLI 交互式登录、状态显示与日志跟踪
- ✅ 虚拟环境、依赖、配置一键自检

## 项目原理

系统由单一的监听器进程组成：

- 使用 Telethon 连接 Telegram 用户会话；
- 按 `.env` 中的白名单 / 黑名单 / 过滤规则处理消息；
- 将格式化后的文本持久化到 SQLite（默认 `data/jtbot.db`），供后续分析或二次同步使用。

## 配置说明

`./start.sh setup` 将在项目根目录创建 `.env`，关键配置如下：

- **Telegram 接入**
  - `TELEGRAM_API_ID` / `TELEGRAM_API_HASH`：在 [my.telegram.org](https://my.telegram.org/apps) 获取。
  - `TELEGRAM_PHONE_NUMBER`：带国家区号的手机号（示例：`+8613800000000`）。
  - `TELEGRAM_PASSWORD`：开启两步验证时填写，否则留空。
  - `TELEGRAM_SESSION_NAME`：可选，自定义 session 文件名前缀。

- **存储与过滤**
  - `DATABASE_PATH`：自定义 SQLite 文件存放位置。
  - `LISTEN_ALL_SUBSCRIBED_CHANNELS`、`CHANNEL_ALLOWLIST`：控制监听范围。
  - `BLOCK_PRIVATE_MESSAGES`、`BLOCKED_SENDER_IDS`、`ENABLE_SENDER_WHITELIST`：过滤私聊和黑名单。

- **代理配置（可选）**
  - `PROXY_TYPE` / `PROXY_HOST` / `PROXY_PORT`：Telethon 连接使用的代理。
  - `USE_PROXY`、`HTTP_PROXY`、`HTTPS_PROXY`：全局 HTTP/HTTPS 代理。

> 修改 `.env` 后执行 `./start.sh restart` 以应用最新配置。

## 文档

- [docs/README.md](docs/README.md) - 详细部署、操作文档
- [docs/REFACTORING_SUMMARY.md](docs/REFACTORING_SUMMARY.md) - 历史重构说明

## 项目结构

```
jt-bot/
├── start.sh                   # 统一启动/管理脚本
├── telegram_collector/        # 核心代码包
│   ├── jt_bot.py              # Telegram 监听器
│   ├── settings.py            # 配置聚合与模型
│   └── config_manager.py      # 路径与环境管理
├── data/                      # 运行时数据（数据库、session 等）
├── logs/                      # 日志输出
├── docs/                      # 使用文档
├── tests/                     # 测试与示例
├── requirements.txt
└── setup.py
```

## 常用命令

```bash
./start.sh setup       # 环境自检与依赖安装
./start.sh auth        # Telegram 登录
./start.sh             # 启动服务并跟踪日志
./start.sh status      # 查看运行状态
./start.sh stop        # 停止监听进程
tail -f logs/jt_bot.log            # 查看 Telegram 监听日志
```

## 注意事项

- 首次运行必须完成 Telegram 登录验证；登录信息会保存在 `telegram_collector/data/sessions/`。
- Session、数据库和日志均可能包含敏感信息，请妥善备份和权限控制。

## License

Private Project
