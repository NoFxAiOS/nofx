#!/bin/sh
# Nginx startup wrapper - substitutes PORT environment variable

# Default PORT to 8080 if not set
export PORT=${PORT:-8080}

echo "🌐 Starting nginx on port $PORT..."
echo "🔍 All environment variables with PORT:"
env | grep -i port || echo "No PORT variables found"

# Generate nginx config from template
envsubst '${PORT}' < /etc/nginx/nginx.conf.template > /etc/nginx/http.d/default.conf

# Show generated config for debugging
echo "📄 Generated nginx config:"
cat /etc/nginx/http.d/default.conf | head -10

# Start nginx
exec nginx -g "daemon off;"
