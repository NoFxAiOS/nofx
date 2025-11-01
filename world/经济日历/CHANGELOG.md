# 更新日志 / Changelog

---

## [v1.0 Minimal] - 2025-11-02

### 🎯 重大简化

彻底精简项目，只保留核心功能：**数据采集 + 数据库存储 + 定时轮询**

### ✅ 保留功能

- ✅ 数据采集 - 从中文 investing.com 抓取经济日历
- ✅ SQLite 存储 - UPSERT 自动去重
- ✅ 定时轮询 - 可配置间隔
- ✅ 增量更新 - 自动更新 actual/forecast/previous
- ✅ 智能代理 - 自动检测和切换
- ✅ 命令行参数 - 灵活配置
- ✅ 后台运行 - nohup/systemd 支持

### ❌ 移除功能（已归档）

- ❌ Rich UI 界面
- ❌ 实时仪表盘
- ❌ Discord 机器人
- ❌ Discord 消息发送
- ❌ 定时调度器
- ❌ 智能自适应轮询
- ❌ 国际化支持
- ❌ 查询工具

### 📊 代码变化

| 项目 | 之前 | 现在 | 变化 |
|------|------|------|------|
| **文件数** | 11 个 | 1 个 | -90.9% |
| **代码行数** | 4,439 行 | 644 行 | -85.5% |
| **Python 文件** | 11 个 | 1 个 | -90.9% |
| **Shell 脚本** | 6 个 | 1 个 | -83.3% |
| **依赖包** | 6 个 | 4 个 | -33.3% |

### 📁 文件结构变化

**之前**:
```
经济日历/
├── 11个Python文件
├── 6个Shell脚本
├── 多个配置文件
└── 大量文档
```

**现在**:
```
经济日历/
├── economic_calendar_minimal.py  (主程序)
├── README.md                     (快速指南)
├── MINIMAL_README.md            (详细文档)
├── start.sh                     (启动脚本)
├── requirements.txt             (依赖)
├── economic_calendar.db         (数据库)
└── archive_all_versions/        (归档)
```

### 🗄️ 归档内容

所有历史版本已归档到 `archive_all_versions/`:

- `original_multifile/` - 原版11个文件
- `archive_versions/` - 精简版（960行）
- `docs/` - 所有历史文档
- `archive/` - 旧版本归档
- `archive_cleaned/` - 清理后的归档
- `logs/` - 日志文件
- `storage/` - 存储文件

### 🎓 学习价值

1. **单文件设计** - 如何组织后台服务
2. **UPSERT 操作** - SQLite 增量更新
3. **网络检测** - 智能代理切换
4. **定时轮询** - 简单的定时任务
5. **命令行工具** - argparse 使用

### 🚀 快速开始

```bash
# 安装依赖
pip install -r requirements.txt

# 启动服务
./start.sh

# 或自定义间隔
python3 economic_calendar_minimal.py --interval 60
```

### 📝 破坏性变更

⚠️ **不兼容的变更**:
- 移除了所有 Discord 功能
- 移除了 Rich UI 界面
- 移除了定时调度器
- 移除了多个 Shell 脚本

💡 **迁移建议**:
- 如需 Discord 功能 → 使用归档的原版
- 如需 UI 界面 → 使用归档的精简版
- 如需定时任务 → 使用归档的原版

### 🐛 Bug 修复

- 无（新版本，全新代码）

### 🔒 安全更新

- 保留了智能代理检测
- 保留了网络超时设置
- 保留了信号处理

---

## [历史版本] - 已归档

### [v0.x Original] - 2025-10-21

完整的多文件版本，包含所有功能：
- 11个 Python 文件
- Rich UI 监控
- Discord 集成
- 定时调度器
- 完整文档

**位置**: `archive_all_versions/original_multifile/`

### [v0.x Standalone] - 2025-11-02

精简版单文件（带 UI）：
- 960 行代码
- Rich UI 界面
- 智能自适应轮询
- 无 Discord 功能

**位置**: `archive_all_versions/archive_versions/`

---

## 未来计划

### 可能的增强

- [ ] 添加 Webhook 通知支持
- [ ] 添加简单的 Web API
- [ ] 添加数据导出功能（CSV/JSON）
- [ ] 添加事件过滤配置
- [ ] 添加多数据源支持

### 不会添加的功能

- ❌ Rich UI（保持轻量）
- ❌ Discord Bot（专注数据采集）
- ❌ 复杂调度（使用系统 cron）

---

**维护者**: 与原项目相同
**最后更新**: 2025-11-02
