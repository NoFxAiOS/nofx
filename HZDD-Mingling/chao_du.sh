#!/bin/bash

# ================= 颜色与样式定义 =================
GREEN='\033[1;32m'        # 荧光绿
CYAN='\033[0;36m'         # 青色 (用于边框)
RED='\033[0;31m'          # 红色 (用于警告)
YELLOW='\033[1;33m'       # 黄色
NC='\033[0m'              # 重置颜色

# ================= 数据库自动检测 =================
# 优先查找 data 目录下的数据库，其次查找上一级目录
if [ -f "../data/data.db" ]; then
    DB_PATH="../data/data.db"
elif [ -f "../data.db" ]; then
    DB_PATH="../data.db"
else
    echo -e "${RED}❌ 错误: 找不到数据库文件 (data.db)${NC}"
    echo "请确认数据库在 ../data/data.db 或 ../data.db"
    exit 1
fi

# ================= 辅助函数：画表格分割线 =================
draw_line() {
    echo -e "${CYAN}+--------------------------------------------------+${NC}"
}

draw_header() {
    echo -e "${CYAN}|${NC} ${GREEN}$1${NC}"
}

# ================= 主逻辑 =================
clear
echo -e "${GREEN}"
echo "  __  __       _        _      Deletion Tool  "
echo " |  \/  | __ _| |_ _ __(_)_  __               "
echo " | |\/| |/ _\` | __| '__| \ \/ /              "
echo " | |  | | (_| | |_| |  | |>  <                "
echo " |_|  |_|\__,_|\__|_|  |_/_/\_\   v4.0 (Email)"
echo -e "${NC}"
echo "----------------------------------------------------"
echo -e "数据库路径: ${YELLOW}$DB_PATH${NC}"
echo "----------------------------------------------------"

# 1. 获取所有用户列表 (修改点：查询 email 而不是 username)
raw_users=$(sqlite3 "$DB_PATH" "SELECT email FROM users WHERE email IS NOT NULL AND email != '';")

if [ -z "$raw_users" ]; then
    echo -e "${RED}❌ 数据库中没有找到任何用户 (email 列为空)。${NC}"
    exit 1
fi

# 将用户存入数组
IFS=$'\n' read -rd '' -a user_array <<< "$raw_users"

draw_line
draw_header "步骤 1/3: 请选择用户 (输入序号)"
draw_line

# 打印用户菜单
i=1
for u in "${user_array[@]}"; do
    printf "${CYAN}|${NC} ${YELLOW}%-3s${NC} : ${GREEN}%s${NC}\n" "$i" "$u"
    ((i++))
done
draw_line

# 用户输入选择
read -p "请输入序号: " user_choice

# 校验输入
if ! [[ "$user_choice" =~ ^[0-9]+$ ]] || [ "$user_choice" -lt 1 ] || [ "$user_choice" -gt "${#user_array[@]}" ]; then
    echo -e "${RED}❌ 无效的选择！退出。${NC}"
    exit 1
fi

# 获取选中的用户邮箱
selected_user="${user_array[$((user_choice-1))]}"
echo -e "✅ 已选择用户: ${GREEN}${selected_user}${NC}"
echo ""

# 2. 获取该用户的 user_id (修改点：根据 email 查 id)
user_id=$(sqlite3 "$DB_PATH" "SELECT id FROM users WHERE email = '$selected_user';")

# 3. 获取该用户的所有策略
# 假设 strategies 表里的字段是 name 和 user_id
raw_strategies=$(sqlite3 "$DB_PATH" "SELECT name FROM strategies WHERE user_id = '$user_id';")

if [ -z "$raw_strategies" ]; then
    echo -e "${RED}⚠️  该用户 [$selected_user] 下没有任何策略。${NC}"
    exit 0
fi

# 将策略存入数组
IFS=$'\n' read -rd '' -a strat_array <<< "$raw_strategies"

draw_line
draw_header "步骤 2/3: 请选择要删除的策略"
draw_line

# 打印策略菜单
j=1
for s in "${strat_array[@]}"; do
    printf "${CYAN}|${NC} ${YELLOW}%-3s${NC} : ${GREEN}%s${NC}\n" "$j" "$s"
    ((j++))
done
draw_line

read -p "请输入序号 (删除对应的策略): " strat_choice

# 校验输入
if ! [[ "$strat_choice" =~ ^[0-9]+$ ]] || [ "$strat_choice" -lt 1 ] || [ "$strat_choice" -gt "${#strat_array[@]}" ]; then
    echo -e "${RED}❌ 无效的选择！退出。${NC}"
    exit 1
fi

# 获取选中的策略名
selected_strat="${strat_array[$((strat_choice-1))]}"

echo ""
echo -e "${RED}================= ⚠️  高危操作警报 ⚠️  =================${NC}"
echo -e "即将删除以下内容："
echo -e "用户: ${GREEN}$selected_user${NC}"
echo -e "策略: ${GREEN}$selected_strat${NC}"
echo -e "${RED}======================================================${NC}"

read -p "确认删除吗? (输入 y 确认): " confirm

if [ "$confirm" == "y" ]; then
    # 执行删除
    sqlite3 "$DB_PATH" "DELETE FROM strategies WHERE user_id = '$user_id' AND name = '$selected_strat';"
    echo ""
    echo -e "${GREEN}✨ 成功！策略已删除，该用户已被“超度”。 ✨${NC}"
else
    echo -e "${YELLOW}🚫 操作已取消。${NC}"
fi
