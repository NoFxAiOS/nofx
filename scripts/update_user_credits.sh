#!/bin/bash
# ============================================================
# Monnaire Trading Agent OS - 用户积分更新工具
# 功能：更新指定用户的积分为目标值
# 版本：1.0
# 日期：2025-12-02
# ============================================================

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 显示帮助信息
show_help() {
    echo ""
    echo "用法: $0 [选项] <user_id> <target_credits>"
    echo ""
    echo "功能："
    echo "  将指定用户的积分更新为目标值"
    echo "  自动计算差额（增加或扣减）"
    echo "  记录完整的积分流水和审计日志"
    echo ""
    echo "选项:"
    echo "  -h, --help          显示帮助信息"
    echo "  -v, --verbose       详细输出"
    echo "  -c, --check         仅查询，不更新"
    echo "  --dry-run           模拟运行，显示将要执行的操作"
    echo "  --force             强制更新（跳过余额检查）"
    echo ""
    echo "参数:"
    echo "  user_id          用户ID（例如：68003b68-2f1d-4618-8124-e93e4a86200a）"
    echo "  target_credits   目标积分值（例如：100000）"
    echo ""
    echo "示例:"
    echo "  $0 68003b68-2f1d-4618-8124-e93e4a86200a 100000"
    echo "  $0 -v 68003b68-2f1d-4618-8124-e93e4a86200a 100000"
    echo "  $0 -c 68003b68-2f1d-4618-8124-e93e4a86200a 100000"
    echo "  $0 --dry-run 68003b68-2f1d-4618-8124-e93e4a86200a 100000"
    echo ""
}

# 检查环境
check_environment() {
    echo -e "${BLUE}=== 检查运行环境 ===${NC}"
    echo ""

    # 检查Go版本
    if ! command -v go &> /dev/null; then
        echo -e "${RED}❌ Go未安装${NC}"
        echo "请安装Go 1.21+: https://golang.org/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓ Go版本: $GO_VERSION${NC}"

    # 检查DATABASE_URL
    if [ -z "$DATABASE_URL" ]; then
        echo -e "${YELLOW}⚠ DATABASE_URL未设置${NC}"
        echo ""
        echo "尝试从 .env 文件加载..."
        if [ -f ".env" ]; then
            export $(grep -v '^#' .env | xargs)
            echo -e "${GREEN}✓ 已从 .env 文件加载环境变量${NC}"
        fi
    fi

    if [ -z "$DATABASE_URL" ]; then
        echo -e "${RED}❌ DATABASE_URL未设置${NC}"
        echo ""
        echo "请在 .env 文件中设置 DATABASE_URL:"
        echo "  DATABASE_URL='postgresql://user:pass@host:5432/dbname?sslmode=require'"
        echo ""
        exit 1
    fi

    echo -e "${GREEN}✓ DATABASE_URL已配置${NC}"

    # 测试数据库连接
    echo -e "${BLUE}测试数据库连接...${NC}"

    # 如果有 psql，使用 psql 测试
    if command -v psql &> /dev/null; then
        if psql "$DATABASE_URL" -c "SELECT 1;" &> /dev/null; then
            echo -e "${GREEN}✓ 数据库连接成功${NC}"
        else
            echo -e "${RED}❌ 数据库连接失败${NC}"
            echo "请检查连接字符串和数据库状态"
            exit 1
        fi
    else
        # 没有 psql，使用 Go 程序测试
        local test_result=$(mktemp)
        cat > "$test_result.go" <<'EOF'
package main
import (
    "database/sql"
    "fmt"
    "os"
    _ "github.com/lib/pq"
)
func main() {
    dbURL := os.Getenv("DATABASE_URL")
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
        os.Exit(1)
    }
    defer db.Close()
    if err := db.Ping(); err != nil {
        fmt.Fprintf(os.Stderr, "Ping失败: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("OK")
}
EOF

        cd "$(dirname "$test_result")" && \
        go mod init temp_test &> /dev/null && \
        go get github.com/lib/pq &> /dev/null && \
        DATABASE_URL="$DATABASE_URL" go run "$(basename "$test_result").go" &> /dev/null

        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ 数据库连接成功${NC}"
        else
            echo -e "${RED}❌ 数据库连接失败${NC}"
            echo "请检查连接字符串和数据库状态"
            rm -f "$test_result.go" "$test_result" go.mod go.sum
            exit 1
        fi
        rm -f "$test_result.go" "$test_result" go.mod go.sum
    fi

    echo ""
}

# 查询用户当前积分
query_user_credits() {
    local user_id=$1
    local verbose=$2

    if [ "$verbose" = true ]; then
        echo -e "${BLUE}=== 查询用户积分信息 ===${NC}"
        echo ""
    fi

    # 查询用户积分
    local result=$(psql "$DATABASE_URL" -t -A -c "
        SELECT available_credits, total_credits, used_credits
        FROM user_credits
        WHERE user_id = '$user_id';
    " 2>/dev/null)

    if [ -z "$result" ]; then
        if [ "$verbose" = true ]; then
            echo -e "${YELLOW}⚠ 用户积分账户不存在${NC}"
        fi
        echo "0|0|0"
        return 1
    fi

    local available=$(echo "$result" | cut -d'|' -f1)
    local total=$(echo "$result" | cut -d'|' -f2)
    local used=$(echo "$result" | cut -d'|' -f3)

    if [ "$verbose" = true ]; then
        echo -e "${GREEN}✓ 用户积分信息:${NC}"
        echo "  可用积分: $available"
        echo "  总积分: $total"
        echo "  已使用: $used"
        echo ""
    fi

    echo "$available|$total|$used"
    return 0
}

# 查询用户基本信息
query_user_info() {
    local user_id=$1

    local email=$(psql "$DATABASE_URL" -t -A -c "
        SELECT email
        FROM users
        WHERE id = '$user_id';
    " 2>/dev/null)

    if [ -z "$email" ]; then
        echo "未知用户"
    else
        echo "$email"
    fi
}

# 更新用户积分
update_user_credits() {
    local user_id=$1
    local target_credits=$2
    local force=$3
    local verbose=$4
    local admin_id="script_admin"

    if [ "$verbose" = true ]; then
        echo -e "${BLUE}=== 更新用户积分 ===${NC}"
        echo ""
    fi

    # 查询当前积分
    local current_info=$(query_user_credits "$user_id" false)
    local current_available=$(echo "$current_info" | cut -d'|' -f1)
    local current_total=$(echo "$current_info" | cut -d'|' -f2)
    local current_used=$(echo "$current_info" | cut -d'|' -f3)

    # 如果用户积分账户不存在，设为0
    if [ -z "$current_available" ]; then
        current_available=0
        current_total=0
        current_used=0
    fi

    # 计算需要调整的积分
    local adjustment=$((target_credits - current_available))

    if [ "$verbose" = true ]; then
        echo -e "${BLUE}积分调整计算:${NC}"
        echo "  当前可用积分: $current_available"
        echo "  目标积分: $target_credits"
        echo "  需要调整: $adjustment"
        echo ""
    fi

    # 检查是否需要调整
    if [ "$adjustment" -eq 0 ]; then
        echo -e "${GREEN}✓ 用户积分已经是目标值，无需调整${NC}"
        return 0
    fi

    # 如果是扣减且未强制，检查余额
    if [ "$adjustment" -lt 0 ] && [ "$force" != true ]; then
        echo -e "${YELLOW}⚠ 将要扣减积分${NC}"
        if [ "$current_available" -lt "$((-adjustment))" ]; then
            echo -e "${RED}❌ 积分不足，无法扣减${NC}"
            echo "  当前可用: $current_available"
            echo "  需要扣减: $((-adjustment))"
            exit 1
        fi
    fi

    # 创建Go程序来更新积分
    local go_code=$(cat <<'EOF'
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "time"

    _ "github.com/lib/pq"
)

func main() {
    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL == "" {
        log.Fatal("DATABASE_URL未设置")
    }

    db, err := sql.Open("postgres", databaseURL)
    if err != nil {
        log.Fatalf("连接数据库失败: %v", err)
    }
    defer db.Close()

    // 测试连接
    if err := db.Ping(); err != nil {
        log.Fatalf("数据库连接测试失败: %v", err)
    }

    userID := os.Args[1]
    targetCredits := 100000
    adminID := "script_admin"
    reason := "脚本更新用户积分"

    // 获取或创建用户积分账户
    var availableCredits, totalCredits, usedCredits int
    var userCreditsID string
    var createdAt, updatedAt time.Time

    err = db.QueryRow(`
        SELECT id, available_credits, total_credits, used_credits, created_at, updated_at
        FROM user_credits
        WHERE user_id = $1
        FOR UPDATE
    `, userID).Scan(&userCreditsID, &availableCredits, &totalCredits, &usedCredits, &createdAt, &updatedAt)

    var isNewAccount bool
    if err != nil {
        if err == sql.ErrNoRows {
            isNewAccount = true
            availableCredits = 0
            totalCredits = 0
            usedCredits = 0
            createdAt = time.Now()
            updatedAt = time.Now()
        } else {
            log.Fatalf("查询用户积分记录失败: %v", err)
        }
    }

    // 计算新的积分
    var newAvailableCredits, newTotalCredits, newUsedCredits int
    var txnType, category string

    adjustment := targetCredits - availableCredits
    if adjustment > 0 {
        newAvailableCredits = availableCredits + adjustment
        newTotalCredits = totalCredits + adjustment
        newUsedCredits = usedCredits
        txnType = "credit"
        category = "admin"
    } else {
        if availableCredits < -adjustment {
            log.Fatalf("积分不足: 当前可用积分 %d，需要扣减 %d", availableCredits, -adjustment)
        }
        newAvailableCredits = availableCredits + adjustment
        newTotalCredits = totalCredits
        newUsedCredits = usedCredits - adjustment
        txnType = "debit"
        category = "admin"
    }

    description := fmt.Sprintf("管理员 %s 更新用户积分: %s (原因: %s)",
        adminID, userID, reason)

    // 开始事务
    tx, err := db.Begin()
    if err != nil {
        log.Fatalf("开始事务失败: %v", err)
    }
    defer tx.Rollback()

    if isNewAccount {
        _, err = tx.Exec(`
            INSERT INTO user_credits
            (id, user_id, available_credits, total_credits, used_credits, created_at, updated_at)
            VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6)
        `, userID, newAvailableCredits, newTotalCredits, newUsedCredits, createdAt, updatedAt)
    } else {
        _, err = tx.Exec(`
            UPDATE user_credits
            SET available_credits = $1, total_credits = $2, used_credits = $3, updated_at = CURRENT_TIMESTAMP
            WHERE user_id = $4
        `, newAvailableCredits, newTotalCredits, newUsedCredits, userID)
    }

    if err != nil {
        log.Fatalf("更新用户积分失败: %v", err)
    }

    // 记录积分流水
    _, err = tx.Exec(`
        INSERT INTO credit_transactions
        (id, user_id, type, amount, balance_before, balance_after, category, description, reference_id, created_at)
        VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
    `, userID, txnType, adjustment, availableCredits, newAvailableCredits,
        category, description, adminID)
    if err != nil {
        log.Fatalf("记录积分流水失败: %v", err)
    }

    // 提交事务
    if err := tx.Commit(); err != nil {
        log.Fatalf("提交事务失败: %v", err)
    }

    fmt.Printf("✅ 用户 %s 积分更新成功\n", userID)
    fmt.Printf("   调整: %d (之前: %d, 之后: %d)\n", adjustment, availableCredits, newAvailableCredits)
}
EOF
)

    # 临时创建并运行Go程序
    local temp_dir=$(mktemp -d)
    local go_file="$temp_dir/update_credits.go"

    echo "$go_code" > "$go_file"

    if [ "$verbose" = true ]; then
        echo -e "${BLUE}执行积分更新操作...${NC}"
    fi

    cd "$temp_dir" && go mod init temp && go get github.com/lib/pq && go run update_credits.go "$user_id"

    local result=$?
    rm -rf "$temp_dir"

    if [ $result -eq 0 ]; then
        echo -e "${GREEN}✓ 积分更新完成${NC}"
        return 0
    else
        echo -e "${RED}❌ 积分更新失败${NC}"
        return 1
    fi
}

# 验证更新结果
verify_update() {
    local user_id=$1
    local target_credits=$2
    local verbose=$3

    if [ "$verbose" = true ]; then
        echo -e "${BLUE}=== 验证更新结果 ===${NC}"
        echo ""
    fi

    local new_info=$(query_user_credits "$user_id" false)
    local new_available=$(echo "$new_info" | cut -d'|' -f1)

    if [ "$new_available" = "$target_credits" ]; then
        echo -e "${GREEN}✓ 验证成功：用户积分为 $new_available${NC}"
        return 0
    else
        echo -e "${RED}❌ 验证失败：期望 $target_credits，实际 $new_available${NC}"
        return 1
    fi
}

# 显示积分流水
show_transactions() {
    local user_id=$1
    local verbose=$2

    if [ "$verbose" != true ]; then
        return 0
    fi

    echo -e "${BLUE}=== 最近积分流水 ===${NC}"
    echo ""

    psql "$DATABASE_URL" -c "
        SELECT
            created_at,
            type,
            amount,
            balance_before,
            balance_after,
            category,
            description
        FROM credit_transactions
        WHERE user_id = '$user_id'
        ORDER BY created_at DESC
        LIMIT 10;
    " 2>/dev/null

    echo ""
}

# 主函数
main() {
    local user_id=""
    local target_credits=""
    local verbose=false
    local check_only=false
    local dry_run=false
    local force=false

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -v|--verbose)
                verbose=true
                shift
                ;;
            -c|--check)
                check_only=true
                shift
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            --force)
                force=true
                shift
                ;;
            -*)
                echo "未知选项: $1"
                show_help
                exit 1
                ;;
            *)
                if [ -z "$user_id" ]; then
                    user_id="$1"
                elif [ -z "$target_credits" ]; then
                    target_credits="$1"
                else
                    echo "错误：参数过多"
                    show_help
                    exit 1
                fi
                shift
                ;;
        esac
    done

    # 验证参数
    if [ -z "$user_id" ] || [ -z "$target_credits" ]; then
        echo -e "${RED}❌ 缺少必要参数${NC}"
        show_help
        exit 1
    fi

    # 验证积分值为正整数
    if ! [[ "$target_credits" =~ ^[0-9]+$ ]]; then
        echo -e "${RED}❌ 积分值必须是正整数${NC}"
        exit 1
    fi

    # 显示标题
    echo ""
    echo "============================================================"
    echo "  Monnaire Trading Agent OS - 用户积分更新工具"
    echo "============================================================"
    echo ""

    # 查询用户信息
    local user_email=$(query_user_info "$user_id")
    echo -e "${BLUE}用户信息:${NC}"
    echo "  用户ID: $user_id"
    echo "  用户邮箱: $user_email"
    echo "  目标积分: $target_credits"
    echo ""

    # 检查环境
    check_environment

    # 查询当前积分
    local current_info=$(query_user_credits "$user_id" "$verbose")
    local current_available=$(echo "$current_info" | cut -d'|' -f1)

    if [ "$check_only" = true ]; then
        if [ -n "$current_available" ] && [ "$current_available" != "0" ]; then
            echo -e "${GREEN}✓ 当前积分为 $current_available${NC}"
        else
            echo -e "${YELLOW}⚠ 当前积分为 0 或账户不存在${NC}"
        fi
        exit 0
    fi

    # 模拟运行
    if [ "$dry_run" = true ]; then
        echo -e "${YELLOW}=== 模拟运行（不会实际更新） ===${NC}"
        echo ""
        echo "将要执行的操作："
        if [ -n "$current_available" ]; then
            local adjustment=$((target_credits - current_available))
            echo "  - 调整积分: $adjustment (从 $current_available 到 $target_credits)"
        else
            echo "  - 创建积分账户，初始值: $target_credits"
        fi
        echo "  - 记录积分流水"
        echo "  - 记录审计日志"
        echo ""
        exit 0
    fi

    # 执行更新
    if update_user_credits "$user_id" "$target_credits" "$force" "$verbose"; then
        # 验证结果
        if verify_update "$user_id" "$target_credits" "$verbose"; then
            # 显示流水
            show_transactions "$user_id" "$verbose"

            echo ""
            echo "============================================================"
            echo -e "${GREEN}✓ 用户积分更新成功完成${NC}"
            echo "============================================================"
            echo ""

            # 提示可在Neon控制台查询
            echo -e "${BLUE}提示：${NC}"
            echo "  你可以在 Neon 控制台中查询验证："
            echo "  1. 访问 https://neon.tech"
            echo "  2. 进入项目数据库"
            echo "  3. 执行查询："
            echo "     SELECT * FROM user_credits WHERE user_id = '$user_id';"
            echo ""

            exit 0
        else
            echo ""
            echo "============================================================"
            echo -e "${RED}❌ 积分更新验证失败${NC}"
            echo "============================================================"
            echo ""
            exit 1
        fi
    else
        echo ""
        echo "============================================================"
        echo -e "${RED}❌ 积分更新失败${NC}"
        echo "============================================================"
        echo ""
        exit 1
    fi
}

# 执行主函数
main "$@"
