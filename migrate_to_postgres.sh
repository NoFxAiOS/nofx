#!/bin/bash

# PostgreSQL数据迁移脚本
# 用于将SQLite数据迁移到PostgreSQL

set -e

echo "🔄 开始数据库迁移..."

# 检查PostgreSQL服务是否运行
echo "📋 检查PostgreSQL服务状态..."
if ! docker-compose ps postgres | grep -q "Up"; then
    echo "⚠️  PostgreSQL服务未运行，正在启动..."
    docker-compose up postgres -d
    echo "⏳ 等待PostgreSQL启动..."
    sleep 10
fi

# 检查连接
echo "🔌 测试数据库连接..."
if ! docker-compose exec postgres pg_isready -U nofx; then
    echo "❌ 无法连接到PostgreSQL，请检查服务状态"
    exit 1
fi

echo "✅ PostgreSQL连接正常"

# 执行数据迁移
echo "📦 执行数据迁移..."
if docker-compose exec -T postgres psql -U nofx -d nofx -f /tmp/migrate_actual_data.sql; then
    echo "✅ 数据迁移成功！"
else
    echo "⚠️  执行迁移脚本..."
    # 将本地文件复制到容器并执行
    docker cp migrate_actual_data.sql $(docker-compose ps -q postgres):/tmp/migrate_actual_data.sql
    docker-compose exec postgres psql -U nofx -d nofx -f /tmp/migrate_actual_data.sql
    echo "✅ 数据迁移完成！"
fi

# 验证数据
echo "🔍 验证迁移结果..."
docker-compose exec postgres psql -U nofx -d nofx -c "
SELECT 'Table Statistics:' as info;
SELECT 
    schemaname,
    tablename, 
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_rows
FROM pg_stat_user_tables 
ORDER BY tablename;
"

echo ""
echo "🎉 数据库迁移完成！"
echo ""
echo "📋 后续步骤："
echo "1. 启动应用: docker-compose up"
echo "2. 验证功能: 访问 http://localhost:3000"
echo "3. 备份原SQLite: mv config.db config.db.backup"
echo ""
echo "🔧 如需回滚到SQLite:"
echo "1. 停止服务: docker-compose down"
echo "2. 删除环境变量: unset POSTGRES_HOST"
echo "3. 恢复备份: mv config.db.backup config.db"
echo "4. 重启: docker-compose up"