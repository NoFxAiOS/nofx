#!/bin/bash
# 启动经济日历数据采集服务
# 用法: ./start.sh [interval]
# 示例: ./start.sh 60 (每60秒轮询一次)

INTERVAL=${1:-300}  # 默认300秒（5分钟）

echo "=================================================="
echo "  启动经济日历数据采集服务"
echo "=================================================="
echo "轮询间隔: $INTERVAL 秒"
echo ""

# 检查依赖
if ! command -v python3 &> /dev/null; then
    echo "❌ 错误: 未找到 python3"
    exit 1
fi

# 检查依赖包
python3 -c "import requests, lxml, pytz, dotenv" 2>/dev/null
if [ $? -ne 0 ]; then
    echo "❌ 错误: 缺少依赖包"
    echo "请运行: pip install -r requirements.txt"
    exit 1
fi

# 启动服务
echo "🚀 启动中..."
python3 economic_calendar_minimal.py --interval $INTERVAL --verbose
