#!/bin/bash

# Fail fast and normalize working directory
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# 内测码生成脚本
# 生成6位不重复的内测码并写入 beta_codes.txt

BETA_CODES_FILE="beta_codes.txt"
COUNT=1
LIST_ONLY=false
CODE_LENGTH=6

# 字符集（避免易混淆字符：0/O, 1/I/l）
CHARSET="23456789abcdefghjkmnpqrstuvwxyz"

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

选项:
    -c COUNT    生成内测码数量 (默认: 1)
    -l          列出现有内测码
    -f FILE     内测码文件路径 (默认: beta_codes.txt)
    -h          显示此帮助信息

示例:
    $0 -c 10                   # 生成10个内测码
    $0 -l                      # 列出现有内测码
    $0 -f custom.txt -c 5      # 在自定义文件中生成5个内测码
EOF
}

# 生成随机内测码
generate_beta_code() {
    local length="$1"
    local charset="$2"
    local code=""
    
    for ((i=0; i<length; i++)); do
        local random_index=$((RANDOM % ${#charset}))
        code+="${charset:$random_index:1}"
    done
    
    echo "$code"
}

# 读取现有内测码
read_existing_codes() {
    local file="$1"
    if [ -f "$file" ]; then
        grep -v '^$' "$file" 2>/dev/null | tr -d ' \t' | grep -v '^#' || true
    fi
}

# 检查内测码是否已存在
code_exists() {
    local code="$1"
    local file="$2"
    if [ -f "$file" ]; then
        grep -Fxq "$code" "$file" 2>/dev/null
    else
        return 1
    fi
}

# 添加内测码到文件
add_code_to_file() {
    local code="$1"
    local file="$2"
    echo "$code" >> "$file"
}

# 验证内测码格式
validate_code() {
    local code="$1"
    # 检查长度
    if [ ${#code} -ne $CODE_LENGTH ]; then
        return 1
    fi
    # 检查字符是否都在允许的字符集中
    if [[ ! "$code" =~ ^[$CHARSET]+$ ]]; then
        return 1
    fi
    return 0
}

# 去重并排序内测码
dedupe_and_sort_codes() {
    local file="$1"
    if [ -f "$file" ]; then
        # 过滤空行和注释，去重并排序
        grep -v '^$' "$file" | grep -v '^#' | sort -u > "${file}.tmp" && mv "${file}.tmp" "$file"
    fi
}

# 解析命令行参数
while getopts "c:lf:h" opt; do
    case $opt in
        c)
            COUNT="$OPTARG"
            if ! [[ "$COUNT" =~ ^[0-9]+$ ]] || [ "$COUNT" -lt 1 ]; then
                echo "错误: count 必须是正整数" >&2
                exit 1
            fi
            ;;
        l)
            LIST_ONLY=true
            ;;
        f)
            BETA_CODES_FILE="$OPTARG"
            ;;
        h)
            show_help
            exit 0
            ;;
        \?)
            echo "无效选项: -$OPTARG" >&2
            echo "使用 -h 查看帮助信息" >&2
            exit 1
            ;;
    esac
done

# 如果是列出现有内测码
if [ "$LIST_ONLY" = true ]; then
    if [ -f "$BETA_CODES_FILE" ]; then
        existing_codes=$(read_existing_codes "$BETA_CODES_FILE")
        if [ -z "$existing_codes" ]; then
            echo "内测码列表为空"
        else
            count=$(echo "$existing_codes" | wc -l | tr -d ' ')
            echo "当前内测码 ($count 个):"
            echo "$existing_codes" | nl -w3 -s'. '
        fi
    else
        echo "内测码文件不存在: $BETA_CODES_FILE"
    fi
    exit 0
fi

# 读取现有内测码
existing_codes=$(read_existing_codes "$BETA_CODES_FILE")

# 生成新内测码
new_codes=()
max_attempts=1000  # 防止无限循环

echo "正在生成 $COUNT 个内测码..."

for ((i=1; i<=COUNT; i++)); do
    attempts=0
    while [ $attempts -lt $max_attempts ]; do
        code=$(generate_beta_code $CODE_LENGTH "$CHARSET")
        
        # 验证格式
        if ! validate_code "$code"; then
            ((attempts++))
            continue
        fi
        
        # 检查是否已存在
        if code_exists "$code" "$BETA_CODES_FILE"; then
            ((attempts++))
            continue
        fi
        
        # 检查是否与本次生成的重复
        duplicate=false
        for existing_code in "${new_codes[@]}"; do
            if [ "$code" = "$existing_code" ]; then
                duplicate=true
                break
            fi
        done
        
        if [ "$duplicate" = false ]; then
            new_codes+=("$code")
            break
        fi
        
        ((attempts++))
    done
    
    if [ $attempts -eq $max_attempts ]; then
        echo "警告: 生成第 $i 个内测码时达到最大尝试次数，可能字符空间不足" >&2
        break
    fi
done

# 检查是否成功生成了内测码
if [ ${#new_codes[@]} -eq 0 ]; then
    echo "未能生成任何新的内测码"
    exit 1
fi

# 添加到文件
for code in "${new_codes[@]}"; do
    add_code_to_file "$code" "$BETA_CODES_FILE"
done

# 去重并排序
dedupe_and_sort_codes "$BETA_CODES_FILE"

echo "成功生成 ${#new_codes[@]} 个内测码:"
printf '  %s\n' "${new_codes[@]}"
echo
echo "内测码文件: $BETA_CODES_FILE"

# 显示当前总数
if [ -f "$BETA_CODES_FILE" ]; then
    total_count=$(read_existing_codes "$BETA_CODES_FILE" | wc -l | tr -d ' ')
    echo "当前内测码总计: $total_count 个"
fi

# 显示文件头部信息（如果是新文件）
if [ ! -s "$BETA_CODES_FILE" ] || [ $(wc -l < "$BETA_CODES_FILE") -eq ${#new_codes[@]} ]; then
    echo
    echo "内测码规则："
    echo "- 长度: $CODE_LENGTH 位"
    echo "- 字符集: 数字 2-9, 小写字母 a-z (排除 0,1,i,l,o 避免混淆)"
    echo "- 每个内测码唯一且不重复"
fi
