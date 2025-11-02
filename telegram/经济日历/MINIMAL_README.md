# 📅 经济日历超精简版 - 仅数据库 + 定时轮询

**版本**: v1.0 Minimal
**文件**: `economic_calendar_minimal.py`
**代码行数**: 644 行
**创建日期**: 2025-11-02

---

## 🎯 这是什么？

这是**极简版本**，只保留最核心的功能：**数据采集 + 数据库存储 + 定时轮询**

### ✅ 包含的功能（最小集）

- ✅ **数据采集** - 从中文 investing.com 抓取经济日历
- ✅ **数据存储** - SQLite 数据库 UPSERT 去重
- ✅ **定时轮询** - 可配置间隔自动更新
- ✅ **智能代理** - 自动检测和切换代理
- ✅ **日志输出** - 简单的控制台日志

### ❌ 去掉的功能（轻量化）

- ❌ Rich UI 界面（纯后台运行）
- ❌ Discord 机器人
- ❌ Discord 消息发送
- ❌ 定时调度器
- ❌ 智能自适应轮询
- ❌ 实时仪表盘
- ❌ 统计面板
- ❌ 国际化支持

---

## 📊 版本对比

| 项目 | 原版 (11文件) | 精简版 (单文件) | **超精简版 (单文件)** |
|------|--------------|----------------|---------------------|
| **文件数量** | 11 个 | 1 个 | 1 个 |
| **代码行数** | 4,439 行 | 960 行 | **644 行** |
| **代码减少** | - | -78.4% | **-85.5%** ✨ |
| **Rich UI** | ✅ | ✅ | ❌ |
| **Discord** | ✅ | ❌ | ❌ |
| **定时轮询** | ✅ | ✅ | ✅ |
| **数据库** | ✅ | ✅ | ✅ |
| **适用场景** | 生产环境 | 学习/测试 | **后台服务** |

---

## 🚀 快速开始

### 1. 安装依赖

```bash
pip install requests lxml pytz python-dotenv
```

### 2. 运行程序

```bash
# 默认配置（5分钟轮询一次）
python3 economic_calendar_minimal.py

# 自定义轮询间隔（60秒）
python3 economic_calendar_minimal.py --interval 60

# 自定义数据范围（未来3天）
python3 economic_calendar_minimal.py --days 3

# 开启详细日志
python3 economic_calendar_minimal.py --verbose

# 后台运行
nohup python3 economic_calendar_minimal.py > calendar.log 2>&1 &
```

### 3. 查看帮助

```bash
python3 economic_calendar_minimal.py --help
```

---

## 📖 使用说明

### 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--interval` | int | 300 | 轮询间隔（秒） |
| `--days` | int | 7 | 获取未来天数 |
| `--verbose` | flag | False | 开启详细日志 |

### 示例

```bash
# 每分钟更新一次，获取未来3天数据，开启详细日志
python3 economic_calendar_minimal.py --interval 60 --days 3 --verbose

# 每小时更新一次（3600秒）
python3 economic_calendar_minimal.py --interval 3600

# 每10秒更新一次（测试用）
python3 economic_calendar_minimal.py --interval 10 --verbose
```

---

## ⚙️ 配置选项

### 环境变量（.env 文件）

```bash
# 数据库路径
DATABASE_URL=economic_calendar.db

# 轮询间隔（秒）- 命令行参数优先
POLL_INTERVAL=300

# 代理模式 (auto/always/never)
PROXY_MODE=auto

# 代理地址
HTTP_PROXY=http://127.0.0.1:9910

# 网络超时（秒）
NETWORK_TEST_TIMEOUT=5
```

### 代理模式说明

- **auto（推荐）**: 自动检测，优先本地网络，失败后使用代理
- **always**: 总是使用代理
- **never**: 从不使用代理

---

## 📋 日志输出

### 普通模式

```
[2025-11-02 10:00:00] [INFO] ============================================================
[2025-11-02 10:00:00] [INFO] 经济日历超精简版 - 启动中...
[2025-11-02 10:00:00] [INFO] ============================================================
[2025-11-02 10:00:00] [INFO] 数据库路径: economic_calendar.db
[2025-11-02 10:00:00] [INFO] 轮询间隔: 300 秒
[2025-11-02 10:00:00] [INFO] 数据范围: 未来 7 天
[2025-11-02 10:00:00] [INFO] 详细日志: 关闭
[2025-11-02 10:00:00] [INFO] ============================================================
[2025-11-02 10:00:00] [INFO] 使用本地网络
[2025-11-02 10:00:00] [INFO] 初始化数据库...
[2025-11-02 10:00:00] [INFO] 数据库已就绪
[2025-11-02 10:00:00] [INFO] 执行首次数据更新...
[2025-11-02 10:00:00] [INFO] 开始获取数据...
[2025-11-02 10:00:05] [INFO] 获取到 245 条事件
[2025-11-02 10:00:05] [INFO] 数据库已更新: 245 条 (总更新次数: 1)
[2025-11-02 10:00:05] [INFO] 事件统计: 总数=245, 高=35, 中=78, 低=132
[2025-11-02 10:00:05] [INFO] 进入轮询循环 (间隔: 300秒)
[2025-11-02 10:00:05] [INFO] 按 Ctrl+C 停止程序
[2025-11-02 10:00:05] [INFO] ============================================================
```

### 详细模式（--verbose）

```
[2025-11-02 10:00:00] [DEBUG] 正在检测网络环境...
[2025-11-02 10:00:00] [DEBUG] 本地网络可用，不使用代理
[2025-11-02 10:00:00] [DEBUG] 获取日期范围: 02/11/2025 - 09/11/2025
[2025-11-02 10:00:05] [DEBUG] 下次更新: 300 秒后
[2025-11-02 10:05:05] [DEBUG] 下次更新: 270 秒后
...
```

---

## 🗄️ 数据库

### 数据库结构

与原版完全相同：

```sql
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    time TEXT,
    zone TEXT,
    currency TEXT,
    event TEXT NOT NULL,
    importance TEXT,
    actual TEXT,
    forecast TEXT,
    previous TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(date, time, zone, event)
);
```

### 查询数据

```bash
# 查看所有数据
sqlite3 economic_calendar.db "SELECT COUNT(*) FROM events;"

# 查看高重要性事件
sqlite3 economic_calendar.db "SELECT date, time, event FROM events WHERE importance = '高' ORDER BY date, time;"

# 查看今日事件
sqlite3 economic_calendar.db "SELECT * FROM events WHERE date = strftime('%d/%m/%Y', 'now', 'localtime');"

# 查看已发布数据
sqlite3 economic_calendar.db "SELECT * FROM events WHERE actual IS NOT NULL AND actual != '' ORDER BY date DESC LIMIT 20;"
```

---

## 🔧 后台运行

### 使用 nohup

```bash
# 后台运行，日志输出到 calendar.log
nohup python3 economic_calendar_minimal.py > calendar.log 2>&1 &

# 查看进程
ps aux | grep economic_calendar_minimal

# 查看日志
tail -f calendar.log

# 停止程序
pkill -f economic_calendar_minimal
```

### 使用 systemd

创建服务文件 `/etc/systemd/system/economic-calendar.service`:

```ini
[Unit]
Description=Economic Calendar Minimal Service
After=network.target

[Service]
Type=simple
User=your_username
WorkingDirectory=/path/to/your/project
ExecStart=/usr/bin/python3 /path/to/economic_calendar_minimal.py --interval 300
Restart=on-failure
RestartSec=10
StandardOutput=append:/var/log/economic-calendar.log
StandardError=append:/var/log/economic-calendar-error.log

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable economic-calendar
sudo systemctl start economic-calendar
sudo systemctl status economic-calendar
```

### 使用 screen

```bash
# 创建 screen 会话
screen -S calendar

# 运行程序
python3 economic_calendar_minimal.py --verbose

# 分离会话：Ctrl+A, D

# 重新连接
screen -r calendar

# 列出所有会话
screen -ls
```

---

## 🎓 代码结构

```python
# ============================================================================
# 1. 依赖检查和导入 (20 行)
# ============================================================================

# ============================================================================
# 2. 命令行参数 (10 行)
# ============================================================================

# ============================================================================
# 3. 全局配置 (50 行)
# ============================================================================

# ============================================================================
# 4. 日志工具 (15 行)
# ============================================================================

# ============================================================================
# 5. 网络工具 (100 行)
# ============================================================================

# ============================================================================
# 6. 数据归一化 (50 行)
# ============================================================================

# ============================================================================
# 7. 数据采集 (150 行)
# ============================================================================

# ============================================================================
# 8. 数据库操作 (80 行)
# ============================================================================

# ============================================================================
# 9. 主程序逻辑 (100 行)
# ============================================================================
```

---

## 🔍 与其他版本对比

### 功能对比表

| 功能 | 原版 (11文件) | 精简版 (960行) | **超精简版 (644行)** |
|------|--------------|---------------|---------------------|
| **数据采集** | ✅ | ✅ | ✅ |
| **数据库存储** | ✅ | ✅ | ✅ |
| **UPSERT 去重** | ✅ | ✅ | ✅ |
| **定时轮询** | ✅ | ✅ | ✅ |
| **智能代理** | ✅ | ✅ | ✅ |
| **Rich UI** | ✅ | ✅ | ❌ |
| **实时仪表盘** | ✅ | ✅ | ❌ |
| **统计面板** | ✅ | ✅ | ❌ |
| **智能自适应** | ✅ | ✅ | ❌ |
| **历史事件** | ✅ | ✅ | ❌ |
| **Discord Bot** | ✅ | ❌ | ❌ |
| **Discord 发送** | ✅ | ❌ | ❌ |
| **定时调度** | ✅ | ❌ | ❌ |
| **国际化** | ✅ | ❌ | ❌ |
| **命令行参数** | ❌ | ❌ | ✅ |
| **后台运行** | ✅ | ⚠️ | ✅ |

---

## ✅ 优点

1. **极简设计** - 只有 644 行代码
2. **后台友好** - 无 UI，适合后台运行
3. **资源占用低** - 无 Rich 库开销
4. **配置灵活** - 支持命令行参数
5. **日志清晰** - 简单的日志输出
6. **易于部署** - 单文件，依赖少

---

## ⚠️ 缺点

1. **无可视化** - 没有实时 UI
2. **功能单一** - 只做数据采集和存储
3. **无自适应** - 固定轮询间隔
4. **无通知** - 没有 Discord 推送

---

## 🎯 适用场景

### ✅ 推荐使用

- ✅ **后台服务** - 作为数据采集守护进程
- ✅ **服务器部署** - 无需 UI 的生产环境
- ✅ **数据源** - 为其他程序提供数据
- ✅ **资源受限** - 内存或 CPU 受限环境
- ✅ **学习研究** - 理解核心数据采集逻辑

### ❌ 不推荐使用

- ❌ **需要实时监控** → 使用精简版（960行）
- ❌ **需要 Discord 推送** → 使用原版（11文件）
- ❌ **需要可视化** → 使用精简版或原版
- ❌ **需要定时任务** → 使用原版

---

## 📚 相关文档

- **原版文档**: [README.md](./README.md)
- **精简版文档**: [archive_versions/STANDALONE_README.md](./archive_versions/STANDALONE_README.md)
- **代码分析**: [CODE_MERGE_ANALYSIS.md](./CODE_MERGE_ANALYSIS.md)
- **清理报告**: [CLEANUP_REPORT.md](./CLEANUP_REPORT.md)

---

## 🔄 版本切换

### 从超精简版切换到精简版

```bash
# 停止超精简版
pkill -f economic_calendar_minimal

# 运行精简版（带 Rich UI）
python3 archive_versions/economic_calendar_standalone.py
```

### 从超精简版切换到原版

```bash
# 停止超精简版
pkill -f economic_calendar_minimal

# 运行原版主程序
./start.sh
```

---

## 🐛 故障排除

### 问题1: 程序无法启动

**检查依赖**:
```bash
pip install requests lxml pytz python-dotenv
```

### 问题2: 网络连接失败

**检查代理配置**:
```bash
# 测试代理
curl -x http://127.0.0.1:9910 https://cn.investing.com

# 强制使用代理
echo "PROXY_MODE=always" >> .env
```

### 问题3: 数据库写入失败

**检查权限**:
```bash
chmod 666 economic_calendar.db
```

### 问题4: 后台运行异常退出

**查看日志**:
```bash
tail -f calendar.log
```

**检查进程**:
```bash
ps aux | grep economic_calendar_minimal
```

---

## 📊 性能数据

### 资源占用（测试环境）

| 指标 | 超精简版 | 精简版 | 原版 |
|------|---------|--------|------|
| **内存占用** | ~30 MB | ~45 MB | ~60 MB |
| **CPU占用** | <1% | <1% | <2% |
| **启动时间** | ~1秒 | ~2秒 | ~3秒 |
| **网络流量** | ~500 KB/次 | ~500 KB/次 | ~500 KB/次 |

### 轮询间隔建议

| 场景 | 建议间隔 | 说明 |
|------|---------|------|
| **生产环境** | 300秒 (5分钟) | 平衡更新频率和资源占用 |
| **开发测试** | 60秒 (1分钟) | 快速验证功能 |
| **高频交易** | 30秒 | 需要实时数据 |
| **低频监控** | 3600秒 (1小时) | 减少网络请求 |

---

## 🎓 学习价值

### 代码亮点

1. **单文件架构** - 如何组织后台服务程序
2. **命令行参数** - argparse 的使用
3. **后台运行** - 信号处理和优雅退出
4. **网络检测** - 智能代理切换
5. **定时轮询** - 简单的定时任务实现
6. **日志系统** - 轻量级日志输出

---

## 📝 更新日志

### v1.0 (2025-11-02)
- ✅ 初始版本
- ✅ 数据采集和数据库存储
- ✅ 定时轮询
- ✅ 智能代理检测
- ✅ 命令行参数支持

---

**创建时间**: 2025-11-02
**版本**: v1.0 Minimal
**代码行数**: 644 行
**适用场景**: 后台数据采集服务
