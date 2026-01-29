#!/bin/bash

# ================= 配置区域 =================
# 自动寻找数据库
if [ -f "../data/data.db" ]; then
    DB_PATH="../data/data.db"
elif [ -f "../data.db" ]; then
    DB_PATH="../data.db"
else
    echo "❌ 错误: 找不到 data.db"
    exit 1
fi

# ================= 颜色定义 =================
GREEN='\033[1;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# ================= 主程序 =================
clear
echo -e "${RED}"
echo "   ___  _            _ _      __  __ _               _     _ "
echo "  / _ \(_)_ __   __ | (_)____ \ \/ /(_) __ _ _ __ __| |__ (_)"
echo " | | | | | '_ \ / _\` | | |_  /  \  /| |/ _\` | '_ \_  / '_ \| |"
echo " | |_| | | | | | (_| | | |/ /   /  \| | (_| | | | / /| | | | |"
echo "  \__\_\_|_| |_|\__, |_|_/___| /_/\_\_|\__,_|_| |_/___|_| |_|_|"
echo "                |___/                                          "
echo "      >>> 闲置/无主策略清理工具 (Qingli Xianzhi) <<<           "
echo -e "${NC}"
echo "----------------------------------------------------"
echo -e "数据库: ${YELLOW}$DB_PATH${NC}"

# 1. 扫描闲置(无主)策略
# 逻辑：查找 user_id 为 NULL，或 user_id 为空字符串，或 user_id 在 users 表里找不到的策略
echo -e "正在扫描数据库中无效的策略记录..."

orphan_sql="SELECT name FROM strategies WHERE user_id IS NULL OR user_id = '' OR user_id NOT IN (SELECT id FROM users);"
orphans=$(sqlite3 "$DB_PATH" "$orphan_sql")

if [ -z "$orphans" ]; then
    echo -e "${GREEN}✅ 完美！数据库非常干净，没有发现闲置或无主的策略。${NC}"
    exit 0
fi

# 2. 列出找到的垃圾数据
echo -e "${RED}⚠️  发现以下无效/无主策略：${NC}"
echo "----------------------------------------------------"
IFS=$'\n' read -rd '' -a orphan_array <<< "$orphans"

i=1
for name in "${orphan_array[@]}"; do
    printf "${RED}[%d] %s${NC}\n" "$i" "$name"
    ((i++))
done
echo "----------------------------------------------------"
echo -e "共发现 ${RED}$((i-1))${NC} 个闲置策略。"

# 3. 确认删除
read -p "⚠️  确认要【彻底删除】这些策略吗? (输入 y 确认): " confirm

if [ "$confirm" == "y" ]; then
    # 执行删除
    sqlite3 "$DB_PATH" "DELETE FROM strategies WHERE user_id IS NULL OR user_id = '' OR user_id NOT IN (SELECT id FROM users);"
    echo -e "${GREEN}🗑️  清理完成！闲置数据已移除。${NC}"
    
    # 4. 自动重启
    echo "----------------------------------------------------"
    echo -e "正在重启服务以刷新缓存..."
    cd ..
    
    if docker compose version &>/dev/null; then
        docker compose restart
    elif command -v docker-compose &>/dev/null; then
        docker-compose restart
    else
        echo -e "${YELLOW}⚠️  未检测到 docker 命令，请手动重启。${NC}"
    fi
    echo -e "${GREEN}✨ 全部搞定！${NC}"
else
    echo -e "${YELLOW}🚫 操作已取消。${NC}"
fi
