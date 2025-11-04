#!/bin/bash

# PostgreSQL数据迁移脚本 - 一键迁移
# 用于将SQLite数据迁移到PostgreSQL

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检测Docker Compose命令
DOCKER_COMPOSE_CMD=""
if command -v "docker-compose" &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
elif command -v "docker" &> /dev/null && docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
else
    echo -e "${RED}❌ 错误：找不到 docker-compose 或 docker compose 命令${NC}"
    echo "请安装 Docker Compose 或确保 Docker 支持 compose 子命令"
    exit 1
fi

echo -e "${BLUE}🔄 开始数据库迁移...${NC}"
echo -e "${BLUE}📋 使用命令: ${DOCKER_COMPOSE_CMD}${NC}"

# 检查必要文件
if [ ! -f "migrate_actual_data.sql" ]; then
    echo -e "${RED}❌ 错误：找不到 migrate_actual_data.sql 文件${NC}"
    echo "请确保在项目根目录执行此脚本"
    exit 1
fi

if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}❌ 错误：找不到 docker-compose.yml 文件${NC}"
    echo "请确保在项目根目录执行此脚本"
    exit 1
fi

# 停止现有服务（避免端口冲突）
echo -e "${YELLOW}🛑 停止现有服务...${NC}"
$DOCKER_COMPOSE_CMD down 2>/dev/null || true

# 启动PostgreSQL和Redis服务
echo -e "${YELLOW}🚀 启动PostgreSQL和Redis服务...${NC}"
$DOCKER_COMPOSE_CMD up postgres redis -d

# 等待服务启动
echo -e "${YELLOW}⏳ 等待服务启动...${NC}"
sleep 15

# 检查PostgreSQL连接
echo -e "${BLUE}🔌 测试数据库连接...${NC}"
max_retries=12
retry_count=0

while [ $retry_count -lt $max_retries ]; do
    if $DOCKER_COMPOSE_CMD exec postgres pg_isready -U nofx > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL连接正常${NC}"
        break
    else
        retry_count=$((retry_count + 1))
        echo -e "${YELLOW}⏳ 等待PostgreSQL启动... (${retry_count}/${max_retries})${NC}"
        sleep 5
    fi
done

if [ $retry_count -eq $max_retries ]; then
    echo -e "${RED}❌ 无法连接到PostgreSQL，请检查服务状态${NC}"
    $DOCKER_COMPOSE_CMD logs postgres
    exit 1
fi

# 复制迁移脚本到容器
echo -e "${BLUE}📦 复制迁移脚本到容器...${NC}"
POSTGRES_CONTAINER=$($DOCKER_COMPOSE_CMD ps -q postgres)
if [ -z "$POSTGRES_CONTAINER" ]; then
    echo -e "${RED}❌ 找不到PostgreSQL容器${NC}"
    exit 1
fi

docker cp migrate_actual_data.sql ${POSTGRES_CONTAINER}:/tmp/migrate_actual_data.sql

# 验证文件复制成功
if ! $DOCKER_COMPOSE_CMD exec postgres test -f /tmp/migrate_actual_data.sql; then
    echo -e "${RED}❌ 迁移脚本复制失败${NC}"
    exit 1
fi

# 执行数据迁移
echo -e "${BLUE}🔄 执行数据迁移...${NC}"
if $DOCKER_COMPOSE_CMD exec postgres env PAGER="" psql -U nofx -d nofx -f /tmp/migrate_actual_data.sql; then
    echo -e "${GREEN}✅ 数据迁移成功！${NC}"
else
    echo -e "${RED}❌ 数据迁移失败${NC}"
    exit 1
fi

# 验证数据
echo -e "${BLUE}🔍 验证迁移结果...${NC}"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT '=== 数据库迁移验证 ===' as info;
SELECT 
    relname as \"表名\", 
    n_live_tup as \"记录数\"
FROM pg_stat_user_tables 
WHERE n_live_tup > 0
ORDER BY relname;
"

# 显示系统配置（简化版本，避免长文本问题）
echo -e "${BLUE}📋 显示关键配置...${NC}"
$DOCKER_COMPOSE_CMD exec postgres psql -U nofx -d nofx --pset pager=off -c "
SELECT COUNT(*) as \"配置项总数\" FROM system_config;
SELECT 'admin_mode: ' || COALESCE((SELECT value FROM system_config WHERE key='admin_mode'), 'N/A') as \"管理员模式\";
SELECT 'beta_mode: ' || COALESCE((SELECT value FROM system_config WHERE key='beta_mode'), 'N/A') as \"内测模式\";
SELECT 'api_server_port: ' || COALESCE((SELECT value FROM system_config WHERE key='api_server_port'), 'N/A') as \"API端口\";
"

echo ""
echo -e "${GREEN}🎉 数据库迁移完成！${NC}"
echo ""
echo -e "${BLUE}📋 后续步骤：${NC}"
echo -e "1. 启动完整应用: ${YELLOW}$DOCKER_COMPOSE_CMD up${NC}"
echo -e "2. 验证功能: 访问 ${YELLOW}http://localhost:3000${NC}"
echo -e "3. 备份原SQLite: ${YELLOW}mv config.db config.db.backup${NC}"
echo ""
echo -e "${BLUE}🔧 如需回滚到SQLite:${NC}"
echo -e "1. 停止服务: ${YELLOW}$DOCKER_COMPOSE_CMD down${NC}"
echo -e "2. 删除环境变量: ${YELLOW}unset POSTGRES_HOST${NC} 或编辑 .env 文件"
echo -e "3. 恢复备份: ${YELLOW}mv config.db.backup config.db${NC}"
echo -e "4. 重启: ${YELLOW}$DOCKER_COMPOSE_CMD up${NC}"
echo ""
echo -e "${GREEN}🚀 PostgreSQL迁移成功！系统已升级到现代化数据库架构${NC}"