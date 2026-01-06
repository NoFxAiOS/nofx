#!/bin/sh
# Nginx startup wrapper - substitutes PORT environment variable

# Default PORT to 8080 if not set
export PORT=${PORT:-8080}

echo "🌐 Starting nginx on port $PORT..."

# Generate nginx config from template
envsubst '${PORT}' < /etc/nginx/nginx.conf.template > /etc/nginx/http.d/default.conf

# Start nginx
exec nginx -g "daemon off;"
