# 🚀 NOFX 启动方式对比

## 快速选择

```bash
# 方式1: 本地开发（推荐开发使用）⭐
./start_local.sh start --dev

# 方式2: Docker 部署（推荐生产使用）🐳
./start_docker.sh start

# 方式3: 原始脚本（兼容保留）
./start.sh start --dev
```

---

## 详细对比

| 特性 | `start_local.sh` | `start_docker.sh` | `start.sh` |
|------|-----------------|------------------|-----------|
| **使用场景** | 本地开发 | 生产部署 | 通用 |
| **Docker 依赖** | ❌ 不需要 | ✅ 需要 | ❌ 不需要 |
| **前端热重载** | ✅ 支持 | ❌ 需重建 | ✅ 支持 |
| **启动速度** | ⚡ 快 | 🐢 较慢 | ⚡ 快 |
| **环境隔离** | ❌ 无 | ✅ 完全隔离 | ❌ 无 |
| **适合场景** | 开发调试 | 服务器部署 | 本地开发 |
| **Paper Trading** | ✅ 自动创建 | ✅ 自动创建 | ✅ 自动创建 |
| **配置复杂度** | 简单 | 简单 | 中等 |
| **日志管理** | 文件 | Docker logs | 文件 |

---

## 常见问题

### Q: 如何选择启动方式？

**开发阶段:**
```bash
./start_local.sh start --dev
```
- 前端代码修改后自动重载
- 快速启动，方便调试
- 直接使用本地工具

**生产部署:**
```bash
./start_docker.sh start
```
- 环境隔离，更安全
- 一键部署，易于管理
- 适合服务器运行

### Q: Paper Trading 在哪里？

所有启动方式都会自动创建 Paper Trading 交易所。

**验证方法:**
```bash
# 检查数据库
sqlite3 config.db "SELECT id, name FROM exchanges WHERE user_id='default';"

# 应该看到
# paper_trading|Paper Trading (Binance Testnet)
```

**如果看不到:**
```bash
# 本地模式
rm config.db && ./start_local.sh start --dev

# Docker 模式
./start_docker.sh rebuild-fresh
```

### Q: 端口被占用怎么办？

**修改端口:**
```bash
# 编辑 .env 文件
NOFX_FRONTEND_PORT=3001  # 改为其他端口
NOFX_BACKEND_PORT=8081   # 改为其他端口
```

**查找占用端口的进程:**
```bash
lsof -i :8080
lsof -i :3000
```

### Q: 如何查看日志？

**本地模式:**
```bash
./start_local.sh logs          # 查看所有日志
./start_local.sh logs backend  # 只看后端
./start_local.sh logs frontend # 只看前端

# 或直接查看文件
tail -f nofx.log
tail -f frontend.log
```

**Docker 模式:**
```bash
./start_docker.sh logs              # 所有容器
./start_docker.sh logs nofx         # 后端容器
./start_docker.sh logs nofx-frontend # 前端容器
```

### Q: 如何停止服务？

```bash
./start_local.sh stop   # 本地模式
./start_docker.sh stop  # Docker 模式
```

---

## 完整命令参考

### start_local.sh

```bash
./start_local.sh start [--dev]    # 启动服务
./start_local.sh stop             # 停止服务
./start_local.sh restart [--dev]  # 重启服务
./start_local.sh status           # 查看状态
./start_local.sh logs [service]   # 查看日志
```

### start_docker.sh

```bash
./start_docker.sh start           # 启动服务
./start_docker.sh stop            # 停止服务
./start_docker.sh restart         # 重启服务
./start_docker.sh status          # 查看状态
./start_docker.sh logs [service]  # 查看日志
./start_docker.sh build           # 重新构建镜像
./start_docker.sh rebuild-fresh   # 完全重建（删除数据库）
./start_docker.sh help            # 帮助信息
```

---

## 文件说明

- `start_local.sh` - 本地开发启动脚本（新）
- `start_docker.sh` - Docker 部署启动脚本（新）
- `start.sh` - 原始启动脚本（保留兼容）
- `setup.sh` - Linux 服务器环境安装脚本
- `START_SCRIPTS_README.md` - 详细文档

---

## 更多帮助

详细文档: [START_SCRIPTS_README.md](./START_SCRIPTS_README.md)

项目文档: [README.md](./README.md)
