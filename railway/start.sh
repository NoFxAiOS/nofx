#!/bin/sh
set -e

# 默认端口
export PORT=${PORT:-3000}
echo "🚀 Starting NOFX on port $PORT..."

# 生成加密密钥（如果没有设置）
if [ -z "$RSA_PRIVATE_KEY" ]; then
    echo "🔐 Generating RSA key..."
    export RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null)
fi

if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    echo "🔐 Generating data encryption key..."
    export DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
fi

# 生成 nginx 配置
echo "📝 Generating nginx config for port $PORT..."
envsubst '${PORT}' < /etc/nginx/nginx.conf.template > /etc/nginx/http.d/default.conf

# 启动后端（后台运行，端口 8081 避免与 nginx 冲突）
echo "🔧 Starting backend on port 8081..."
API_SERVER_PORT=8081 /app/nofx &
BACKEND_PID=$!

# 等待后端启动
sleep 3

# 检查后端是否启动成功
if ! kill -0 $BACKEND_PID 2>/dev/null; then
    echo "❌ Backend failed to start"
    exit 1
fi

echo "✅ Backend started (PID: $BACKEND_PID)"

# 启动 nginx（前台运行）
echo "🌐 Starting nginx on port $PORT..."
exec nginx -g "daemon off;"
