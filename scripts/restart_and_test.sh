#!/bin/bash

echo "=================================="
echo "NOFX 后端重启和测试脚本"
echo "=================================="

# 1. 停止旧进程
echo ""
echo "1️⃣  停止旧进程..."
pkill -f "bin/nofx" || echo "  没有运行中的进程"
sleep 2

# 2. 清理旧数据
echo ""
echo "2️⃣  清理测试数据..."
sqlite3 data/data.db "DELETE FROM trader_fills; DELETE FROM trader_orders;"
echo "  ✅ trader_orders 和 trader_fills 表已清空"

# 3. 验证数据库已清空
ORDERS_COUNT=$(sqlite3 data/data.db "SELECT COUNT(*) FROM trader_orders")
FILLS_COUNT=$(sqlite3 data/data.db "SELECT COUNT(*) FROM trader_fills")
echo "  验证: trader_orders=$ORDERS_COUNT, trader_fills=$FILLS_COUNT"

# 4. 启动新进程
echo ""
echo "3️⃣  启动新编译的后端服务..."
if [ ! -f "bin/nofx" ]; then
    echo "  ❌ bin/nofx 不存在，请先运行 go build -o bin/nofx ."
    exit 1
fi

nohup ./bin/nofx > data/nofx_$(date +%Y-%m-%d).log 2>&1 &
NOFX_PID=$!
echo "  ✅ 后端已启动 (PID: $NOFX_PID)"

# 5. 等待服务启动
echo ""
echo "4️⃣  等待服务启动..."
sleep 3

# 6. 验证进程运行
if ps -p $NOFX_PID > /dev/null; then
    echo "  ✅ 后端进程运行正常 (PID: $NOFX_PID)"
else
    echo "  ❌ 后端进程启动失败，请检查日志"
    tail -20 data/nofx_$(date +%Y-%m-%d).log
    exit 1
fi

echo ""
echo "=================================="
echo "✅ 重启完成！"
echo "=================================="
echo ""
echo "📝 下一步操作："
echo "  1. 访问前端页面"
echo "  2. 执行一次平仓操作（手动或AI）"
echo "  3. 等待 10 秒（让 pollLighterTradeHistory 完成）"
echo "  4. 检查数据库："
echo "     sqlite3 data/data.db \"SELECT id, status, avg_fill_price, filled_quantity FROM trader_orders\""
echo "  5. 刷新图表页面，应该能看到 B/S 标记"
echo ""
echo "📊 实时日志查看："
echo "  tail -f data/nofx_$(date +%Y-%m-%d).log | grep -E 'Order recorded|Found matching trade|Fill recorded'"
echo ""
