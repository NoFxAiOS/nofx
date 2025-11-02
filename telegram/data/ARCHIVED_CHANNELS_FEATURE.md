# 归档频道过滤功能说明

## 功能概述

新增了 **只监听归档频道** 的功能，允许你通过配置文件控制是否只收集 Telegram 中归档文件夹里的频道/群组消息。

---

## 配置说明

### 新增配置项

在 `.env` 文件中添加了新的配置项：

```bash
LISTEN_ARCHIVED_ONLY=false
```

### 配置组合说明

| LISTEN_ALL_SUBSCRIBED_CHANNELS | LISTEN_ARCHIVED_ONLY | 实际行为 |
|-------------------------------|---------------------|---------|
| `false` | `false` | 只监听 `CHANNEL_ALLOWLIST` 中的频道（不管是否归档） |
| `false` | `true` | 只监听 `CHANNEL_ALLOWLIST` 中的频道（不管是否归档） |
| `true` | `false` | 监听所有订阅的频道/群组（主界面 + 归档） |
| `true` | `true` | **只监听归档文件夹中的频道/群组** ✅ |

---

## 使用示例

### 示例 1: 只监听归档频道

如果你想**只收集归档频道的消息**：

```bash
# .env 配置
LISTEN_ALL_SUBSCRIBED_CHANNELS=true    # 启用自动监听
LISTEN_ARCHIVED_ONLY=true              # 只监听归档的
CHANNEL_ALLOWLIST=                      # 留空（不使用白名单）
```

**效果**：系统只会监听你在 Telegram 中归档（Archive）的频道和群组。

### 示例 2: 监听所有频道（默认）

如果你想**监听所有订阅的频道**（不区分归档）：

```bash
# .env 配置
LISTEN_ALL_SUBSCRIBED_CHANNELS=true    # 启用自动监听
LISTEN_ARCHIVED_ONLY=false             # 不过滤归档
CHANNEL_ALLOWLIST=                      # 留空
```

**效果**：监听所有订阅的频道，包括主界面和归档的。

### 示例 3: 使用白名单（手动指定）

如果你想**手动指定监听的频道**：

```bash
# .env 配置
LISTEN_ALL_SUBSCRIBED_CHANNELS=false   # 关闭自动监听
LISTEN_ARCHIVED_ONLY=false             # 无效（因为使用白名单）
CHANNEL_ALLOWLIST=-1001662298104,-1001500918158  # 手动指定频道ID
```

**效果**：只监听白名单中的频道，`LISTEN_ARCHIVED_ONLY` 配置无效。

---

## 技术实现

### 核心逻辑

在 `jt_bot.py:1209-1248` 的 `get_subscribed_channels()` 方法中：

```python
# 如果启用了只监听归档，则只获取归档对话（folder_id=1）
if hasattr(config, "listen_archived_only") and config.listen_archived_only:
    LOGGER.info("已启用只监听归档频道模式 (LISTEN_ARCHIVED_ONLY=true)")
    dialogs = await self.client.get_dialogs(folder=1)  # 🔑 关键：folder=1 表示归档
else:
    dialogs = await self.client.get_dialogs()
```

### Telegram API 说明

根据 Telethon API：
- `folder_id=0` 或 `None`: 主界面对话
- `folder_id=1`: 归档对话（Archive）
- `folder_id>1`: 用户自定义文件夹（如果有）

---

## 日志输出

启用归档模式后，日志会显示：

```
2025-11-02 18:40:22 │ INFO │ 已启用只监听归档频道模式 (LISTEN_ARCHIVED_ONLY=true)
2025-11-02 18:40:22 │ INFO │ 配置为监听归档频道，正在获取频道列表...
2025-11-02 18:40:22 │ DEBUG │ 发现频道: 华尔街见闻 (@wallstreetcn) [ID: -1001234567890] 📂 [归档]
2025-11-02 18:40:22 │ INFO │ 成功获取到 15 个订阅频道
```

控制台输出：

```
📂 归档频道列表:
 1. 华尔街见闻                     (@wallstreetcn        ) 📂
 2. 金十数据                       (@jin10com            ) 📂
 3. 彭博社                         (@bloomberg           ) 📂
   ... 和其他 12 个频道
```

---

## 如何在 Telegram 中归档频道

1. 打开 Telegram 客户端
2. 在聊天列表中找到你想归档的频道/群组
3. **桌面版**: 右键点击 → 选择 "归档" (Archive)
4. **移动版**: 长按聊天 → 选择 "归档" (Archive)
5. 归档的频道会移动到 "归档聊天" 文件夹

---

## 验证归档功能

### 方法 1: 查看启动日志

启动程序后查看日志输出：

```bash
./start.sh
```

检查是否显示 "已启用只监听归档频道模式"。

### 方法 2: 运行测试脚本（需要停止正在运行的服务）

```bash
# 1. 停止正在运行的服务
./start.sh stop

# 2. 运行测试脚本
source .venv/bin/activate
python test_archived_feature.py

# 3. 重新启动服务
./start.sh
```

测试脚本会显示：
- 所有订阅的频道数量
- 归档频道数量
- 主界面频道数量
- 每个频道的归档状态

---

## 注意事项

1. **数据库锁定**:
   - 如果程序正在运行，测试脚本会因数据库锁定而失败
   - 需要先停止正在运行的服务再测试

2. **配置优先级**:
   - 当 `LISTEN_ALL_SUBSCRIBED_CHANNELS=false` 时，`LISTEN_ARCHIVED_ONLY` 无效
   - 白名单模式优先级高于归档过滤

3. **实时性**:
   - 归档状态在程序启动时获取
   - 运行时在 Telegram 中归档/取消归档频道不会立即生效
   - 需要重启程序以应用新的归档状态

4. **兼容性**:
   - 该功能使用 Telethon 的 `get_dialogs(folder=1)` API
   - 需要 Telethon 版本 >= 1.24.0（当前版本已满足）

---

## 故障排查

### 问题 1: 没有监听到归档频道的消息

**检查清单**:
1. 确认 `.env` 中 `LISTEN_ARCHIVED_ONLY=true`
2. 确认 `LISTEN_ALL_SUBSCRIBED_CHANNELS=true`
3. 确认频道已在 Telegram 中归档
4. 重启程序以刷新归档状态

### 问题 2: 测试脚本报错 "database is locked"

**解决方法**:
```bash
# 停止正在运行的服务
./start.sh stop

# 或者查找并杀死进程
ps aux | grep jt_bot
kill <PID>

# 然后重新运行测试
```

### 问题 3: 归档频道数量为 0

**可能原因**:
1. Telegram 账号没有归档任何频道
2. 归档的都是私聊/机器人，不是频道/群组
3. API 权限问题

**解决方法**:
在 Telegram 中手动归档一些频道，然后重启程序。

---

## 代码变更摘要

### 修改的文件

1. **`.env`** (第 23 行)
   - 新增 `LISTEN_ARCHIVED_ONLY=false` 配置项

2. **`telegram_collector/jt_bot.py`**
   - 第 418-421 行: 添加 `listen_archived_only` 配置读取
   - 第 1209-1248 行: 修改 `get_subscribed_channels()` 支持归档过滤
   - 第 1258-1290 行: 更新日志输出，显示归档模式状态

3. **新增文件**
   - `test_archived_feature.py`: 归档功能测试脚本
   - `ARCHIVED_CHANNELS_FEATURE.md`: 功能说明文档（本文件）

---

## 未来扩展

可能的功能扩展方向：

1. **支持自定义文件夹**:
   - Telegram 支持多个自定义文件夹（folder_id > 1）
   - 可以添加 `LISTEN_FOLDER_ID` 配置指定监听特定文件夹

2. **动态归档检测**:
   - 定期检查频道归档状态变化
   - 无需重启即可应用新的归档设置

3. **归档状态持久化**:
   - 将频道归档状态保存到数据库
   - 支持归档历史记录查询

---

## 联系与反馈

如有问题或建议，请提交 Issue 或 Pull Request。

---

**最后更新**: 2025-11-02
**版本**: v1.0.0
