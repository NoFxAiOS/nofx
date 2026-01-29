#!/bin/bash

# ================= 配置区域 =================
ADMIN_EMAIL="haotianda6@gmail.com"
DB_PATH="../data.db"
# 如果数据库在 data 目录下，自动修正
if [ -f "../data/data.db" ]; then DB_PATH="../data/data.db"; fi

# ================= 颜色定义 =================
GREEN='\033[1;32m'
CYAN='\033[0;36m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# ================= 主程序 =================
clear
echo -e "${GREEN}>>> 策略授权分发系统 (修复版) <<<${NC}"
echo "----------------------------------------"

# --- 1. 获取管理员 ID ---
echo -e "正在验证管理员账户: ${YELLOW}$ADMIN_EMAIL${NC}"
admin_id=$(sqlite3 "$DB_PATH" "SELECT id FROM users WHERE email = '$ADMIN_EMAIL';")

if [ -z "$admin_id" ]; then
    echo -e "${RED}❌ 错误: 找不到管理员账户。${NC}"
    exit 1
fi
echo -e "✅ 管理员 ID: $admin_id"
echo "----------------------------------------"

# --- 2. 选择要分发的策略 ---
# 获取策略列表
raw_strats=$(sqlite3 "$DB_PATH" "SELECT name FROM strategies WHERE user_id = '$admin_id';")
if [ -z "$raw_strats" ]; then echo -e "${RED}❌ 管理员名下无策略${NC}"; exit 1; fi

# 存入数组
IFS=$'\n' read -rd '' -a strat_array <<< "$raw_strats"

echo -e "${CYAN}|${NC} 请选择要【分发】的策略:"
i=1
for s in "${strat_array[@]}"; do
    printf "   [${GREEN}%d${NC}] %s\n" "$i" "$s"
    ((i++))
done
read -p "请输入序号: " s_choice

# 校验
if ! [[ "$s_choice" =~ ^[0-9]+$ ]] || [ "$s_choice" -lt 1 ] || [ "$s_choice" -gt "${#strat_array[@]}" ]; then
    echo -e "${RED}❌ 选择无效${NC}"; exit 1; 
fi
source_strat_name="${strat_array[$((s_choice-1))]}"
echo -e "📜 已选策略: ${YELLOW}$source_strat_name${NC}"
echo "----------------------------------------"

# --- 3. 选择接收者 (修复点：直接逻辑，不封装函数) ---
# 获取除了管理员以外的所有用户
raw_users=$(sqlite3 "$DB_PATH" "SELECT email FROM users WHERE email IS NOT NULL AND email != '' AND email != '$ADMIN_EMAIL';")
if [ -z "$raw_users" ]; then echo -e "${RED}❌ 没有其他用户${NC}"; exit 1; fi

IFS=$'\n' read -rd '' -a user_array <<< "$raw_users"

echo -e "${CYAN}|${NC} 请选择【接收者】:"
j=1
for u in "${user_array[@]}"; do
    printf "   [${GREEN}%d${NC}] %s\n" "$j" "$u"
    ((j++))
done
read -p "请输入序号: " u_choice

# 校验
if ! [[ "$u_choice" =~ ^[0-9]+$ ]] || [ "$u_choice" -lt 1 ] || [ "$u_choice" -gt "${#user_array[@]}" ]; then
    echo -e "${RED}❌ 选择无效${NC}"; exit 1; 
fi
target_email="${user_array[$((u_choice-1))]}"

# --- 4. 获取目标 ID (关键步骤) ---
target_id=$(sqlite3 "$DB_PATH" "SELECT id FROM users WHERE email = '$target_email';")

if [ -z "$target_id" ]; then
    echo -e "${RED}❌ 严重错误: 无法获取用户 [$target_email] 的 ID。${NC}"
    exit 1
fi
echo -e "✅ 目标用户: ${YELLOW}$target_email${NC} (ID: $target_id)"

# --- 5. 执行数据库插入 ---
echo "----------------------------------------"
echo -e "正在写入数据库..."

new_uuid=$(cat /proc/sys/kernel/random/uuid)
new_name="${source_strat_name} [授权版]"
new_desc="【授权使用】源码已隐藏，禁止修改。"

# 这里的 SQL 显式使用了 $target_id
sqlite3 "$DB_PATH" <<SQL_END
INSERT INTO strategies (
    id, user_id, name, description, 
    config, is_active, is_default, is_public, 
    config_visible, created_at, updated_at
)
SELECT 
    '$new_uuid', 
    '$target_id', 
    '$new_name', 
    '$new_desc',
    config, 
    0, 0, 0, 
    0, 
    datetime('now'), datetime('now')
FROM strategies 
WHERE user_id = '$admin_id' AND name = '$source_strat_name';
SQL_END

if [ $? -eq 0 ]; then
    echo -e "${GREEN}🎉 授权成功！${NC}"
else
    echo -e "${RED}❌ 数据库写入失败${NC}"
    exit 1
fi

# --- 6. 兼容性重启 ---
echo "----------------------------------------"
echo -e "正在重启系统..."
cd ..

# 尝试 docker compose (新版)
if docker compose version &>/dev/null; then
    docker compose restart
# 尝试 docker-compose (旧版)
elif command -v docker-compose &>/dev/null; then
    docker-compose restart
else
    echo -e "${YELLOW}⚠️  未检测到 docker compose 命令，请手动重启容器。${NC}"
fi
echo -e "${GREEN}✅ 完成！${NC}"
