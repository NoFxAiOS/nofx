# 项目目录结构说明

## 新的目录结构

项目已优化为清晰的前后端分离结构：

```
nofx/
├── backend/                    # 后端Go代码
│   ├── cmd/
│   │   └── server/
│   │       └── main.go        # 应用入口
│   ├── internal/              # 内部包（不对外暴露）
│   │   ├── api/              # API路由和处理器
│   │   ├── auth/             # 认证授权
│   │   ├── config/           # 配置管理
│   │   ├── manager/          # 交易员管理
│   │   ├── market/           # 市场数据
│   │   ├── trader/           # 交易逻辑
│   │   ├── pool/             # 币种池
│   │   ├── decision/         # AI决策引擎
│   │   ├── logger/           # 日志系统
│   │   ├── bootstrap/        # 启动引导
│   │   ├── mcp/              # MCP客户端
│   │   └── crypto/           # 加密功能
│   ├── prompts/              # AI提示词模板
│   ├── go.mod                # Go模块定义
│   └── go.sum                # Go依赖锁定
│
├── frontend/                  # 前端代码（React/TypeScript）
│   ├── src/                  # 源代码
│   ├── public/              # 静态资源
│   ├── package.json         # 前端依赖
│   └── ...
│
├── configs/                  # 配置文件目录
│   ├── config.json          # 主配置文件
│   ├── config.json.example  # 配置模板
│   ├── config.db            # SQLite数据库
│   ├── beta_codes.txt       # 内测码文件
│   └── nginx/               # Nginx配置
│       └── nginx.conf
│
├── scripts/                  # 脚本文件
│   ├── start.sh             # 启动脚本
│   ├── pm2.sh               # PM2部署脚本
│   ├── generate_beta_code.sh # 内测码生成
│   ├── deploy_encryption.sh  # 加密部署脚本
│   └── migrate_encryption.go # 数据迁移脚本
│
├── docker/                   # Docker相关
│   ├── docker-compose.yml    # Docker Compose配置
│   ├── Dockerfile.backend    # 后端镜像
│   └── Dockerfile.frontend   # 前端镜像
│
├── docs/                     # 文档
├── decision_logs/           # 决策日志（运行时生成）
└── ...其他根目录文件
```

## 主要变更

### 1. 后端代码组织
- 所有Go代码移至 `backend/` 目录
- 主程序入口：`backend/cmd/server/main.go`
- 内部包统一在 `backend/internal/` 下
- 导入路径从 `nofx/xxx` 改为 `nofx/backend/internal/xxx`

### 2. 前端代码
- 从 `web/` 移至 `frontend/`
- 保持原有结构和功能不变

### 3. 配置文件
- 所有配置文件移至 `configs/` 目录
- 包括：`config.json`, `config.db`, `beta_codes.txt`, `nginx.conf`

### 4. 脚本文件
- 所有脚本移至 `scripts/` 目录
- 已更新所有路径引用

### 5. Docker配置
- Docker相关文件移至 `docker/` 目录
- 已更新所有路径映射和环境变量

