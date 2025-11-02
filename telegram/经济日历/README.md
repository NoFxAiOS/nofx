# 📅 经济日历 - 数据采集服务

**极简版本** - 只保留核心功能：数据采集 + 数据库存储 + 定时轮询

---

## 🚀 快速开始

### 1. 安装依赖

```bash
pip install -r requirements.txt
```

### 2. 启动服务

```bash
# 方式1: 直接运行（默认5分钟轮询）
python3 economic_calendar_minimal.py

# 方式2: 自定义间隔（60秒）
python3 economic_calendar_minimal.py --interval 60

# 方式3: 后台运行
nohup python3 economic_calendar_minimal.py > calendar.log 2>&1 &
```

### 3. 查看数据

```bash
# 查看数据总数
sqlite3 economic_calendar.db "SELECT COUNT(*) FROM events;"

# 查看最新10条事件
sqlite3 economic_calendar.db "SELECT date, time, event FROM events ORDER BY date DESC LIMIT 10;"
```

---

## 📊 功能特性

- ✅ **数据采集** - 从中文 investing.com 抓取经济日历
- ✅ **数据库存储** - SQLite + UPSERT 自动去重
- ✅ **定时轮询** - 可配置间隔（默认300秒）
- ✅ **增量更新** - 自动更新 actual/forecast/previous
- ✅ **智能代理** - 自动检测网络和代理切换
- ✅ **后台运行** - 支持 nohup/systemd

---

## ⚙️ 配置选项

### 命令行参数

```bash
python3 economic_calendar_minimal.py --help
```

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--interval` | 轮询间隔（秒） | 300 |
| `--days` | 获取未来天数 | 7 |
| `--verbose` | 详细日志 | False |

### 环境变量（.env 文件）

```bash
# 代理模式 (auto/always/never)
PROXY_MODE=auto

# 代理地址
HTTP_PROXY=http://127.0.0.1:9910

# 数据库路径
DATABASE_URL=economic_calendar.db
```

---

## 📖 使用示例

### 基本用法

```bash
# 默认配置（5分钟轮询）
python3 economic_calendar_minimal.py

# 每分钟更新
python3 economic_calendar_minimal.py --interval 60 --verbose

# 每小时更新
python3 economic_calendar_minimal.py --interval 3600
```

### 后台运行

```bash
# 使用 nohup
nohup python3 economic_calendar_minimal.py > calendar.log 2>&1 &

# 查看日志
tail -f calendar.log

# 停止服务
pkill -f economic_calendar_minimal
```

### 数据库查询

```bash
# 查看所有高重要性事件
sqlite3 economic_calendar.db "SELECT * FROM events WHERE importance = '高';"

# 查看今日事件
sqlite3 economic_calendar.db "SELECT * FROM events WHERE date = strftime('%d/%m/%Y', 'now', 'localtime');"

# 查看已发布数据
sqlite3 economic_calendar.db "SELECT * FROM events WHERE actual IS NOT NULL;"
```

---

## 🗄️ 数据库结构

```sql
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,              -- 日期 (dd/mm/yyyy)
    time TEXT,                       -- 时间 (HH:MM 或 "全天")
    zone TEXT,                       -- 地区
    currency TEXT,                   -- 货币
    event TEXT NOT NULL,             -- 事件名称
    importance TEXT,                 -- 重要性 (高/中/低)
    actual TEXT,                     -- 实际值
    forecast TEXT,                   -- 预期值
    previous TEXT,                   -- 前值
    created_at TEXT NOT NULL,        -- 创建时间
    updated_at TEXT NOT NULL,        -- 更新时间
    UNIQUE(date, time, zone, event)  -- 唯一约束
);
```

**增量更新机制**:
- 使用 `UPSERT` (INSERT ... ON CONFLICT DO UPDATE)
- 相同事件自动更新而不是重复插入
- 自动更新 `actual`, `forecast`, `previous` 字段

---

## 📁 项目结构

```
经济日历/
├── economic_calendar_minimal.py    # 主程序 (644行)
├── README.md                       # 本文档
├── MINIMAL_README.md              # 详细文档
├── requirements.txt               # 依赖列表
├── .env.example                   # 配置模板
├── economic_calendar.db           # SQLite 数据库
└── archive_all_versions/          # 历史版本归档
    ├── original_multifile/        # 原版11文件
    ├── archive_versions/          # 精简版
    └── docs/                      # 文档
```

---

## 🔧 依赖说明

```
requests     # HTTP 请求
lxml         # HTML 解析
pytz         # 时区处理
python-dotenv # 环境变量
```

安装：
```bash
pip install -r requirements.txt
```

---

## 📝 日志示例

```
[2025-11-02 10:00:00] [INFO] 经济日历超精简版 - 启动中...
[2025-11-02 10:00:00] [INFO] 数据库路径: economic_calendar.db
[2025-11-02 10:00:00] [INFO] 轮询间隔: 300 秒
[2025-11-02 10:00:01] [INFO] 使用本地网络
[2025-11-02 10:00:01] [INFO] 数据库已就绪
[2025-11-02 10:00:05] [INFO] 获取到 245 条事件
[2025-11-02 10:00:05] [INFO] 数据库已更新: 245 条
[2025-11-02 10:00:05] [INFO] 事件统计: 总数=245, 高=35, 中=78, 低=132
[2025-11-02 10:00:05] [INFO] 进入轮询循环 (间隔: 300秒)
```

---

## 🐛 故障排除

### 问题1: 网络连接失败

```bash
# 检查代理
curl -x http://127.0.0.1:9910 https://cn.investing.com

# 强制使用代理
echo "PROXY_MODE=always" > .env
```

### 问题2: 数据库写入失败

```bash
# 检查权限
chmod 666 economic_calendar.db

# 重建数据库
rm economic_calendar.db
python3 economic_calendar_minimal.py
```

### 问题3: 查看运行状态

```bash
# 查看进程
ps aux | grep economic_calendar_minimal

# 查看日志
tail -f calendar.log
```

---

## 📚 详细文档

- **MINIMAL_README.md** - 完整使用指南
- **archive_all_versions/docs/** - 历史文档归档

---

## 🔄 版本历史

- **v1.0 (当前)** - 极简版，只保留核心轮询功能
- **历史版本** - 已归档到 `archive_all_versions/`

---

## 📄 许可证

与原项目相同

---

**最后更新**: 2025-11-02
