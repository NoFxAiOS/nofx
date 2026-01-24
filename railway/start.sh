#!/bin/sh
set -e

# Railway 会设置 PORT 环境变量
export PORT=${PORT:-8080}
echo "🚀 Starting NOFX on port $PORT..."

# 生成加密密钥（如果没有设置）
# RSA 用 \n 代替换行，避免 Railway 等平台对换行的处理导致 invalid PEM
if [ -z "$RSA_PRIVATE_KEY" ]; then
    export RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null | tr '\n' '#' | sed 's/#/\\n/g')
fi
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    export DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
fi

# 生成 nginx 配置
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

# 启动后端（端口 8081）
API_SERVER_PORT=8081 /app/nofx &
sleep 4

# 检查后端是否在 8081 响应（若崩溃会在这里暴露）
if ! wget -q -O- --timeout=3 http://127.0.0.1:8081/api/health >/dev/null 2>&1; then
    echo "❌ Backend not responding on 8081. Set in Railway: JWT_SECRET, DATA_ENCRYPTION_KEY, RSA_PRIVATE_KEY. Check deploy logs for nofx Fatal/panic."
    exit 1
fi

# 启动 nginx（后台）
nginx

echo "✅ NOFX started successfully"

# 保持容器运行
tail -f /dev/null
