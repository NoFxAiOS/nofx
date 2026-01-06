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

# 生成 nginx 配置（直接写入，避免 envsubst 问题）
echo "📝 Generating nginx config for port $PORT..."
cat > /etc/nginx/http.d/default.conf << NGINX_EOF
server {
    listen $PORT;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081/api/;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_connect_timeout 300s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    location /health {
        return 200 'OK';
        add_header Content-Type text/plain;
    }
}
NGINX_EOF
echo "✅ Nginx config generated"
cat /etc/nginx/http.d/default.conf

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
