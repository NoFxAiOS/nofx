#!/bin/bash

# 将beta_codes.txt刷到PostgreSQL数据库
echo "🎟️ 导入beta_codes.txt到数据库"

# 检查文件
if [ ! -f "beta_codes.txt" ]; then
    echo "❌ 找不到beta_codes.txt文件"
    exit 1
fi

# 检测docker命令
if command -v "docker-compose" &> /dev/null; then
    DOCKER_CMD="docker-compose"
else
    DOCKER_CMD="docker compose"
fi

# 统计数量
TOTAL=$(cat beta_codes.txt | wc -l)
echo "📊 文件中共有 $TOTAL 个内测码"

# 生成SQL
cat > import.sql << EOF
INSERT INTO beta_codes (code) VALUES
EOF

# 读取每行并生成INSERT语句
cat beta_codes.txt | while read line; do
    if [ -n "$line" ]; then
        echo "('$line')," >> import.sql
    fi
done

# 移除最后的逗号并添加冲突处理
sed -i '$ s/,$//' import.sql
echo "ON CONFLICT (code) DO NOTHING;" >> import.sql

# 执行导入
echo "🔄 导入到数据库..."
$DOCKER_CMD exec -T postgres psql -U nofx -d nofx < import.sql

echo "✅ 导入完成（重复的已跳过）"

# 清理
rm import.sql
