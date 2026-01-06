#!/bin/sh
# Backend startup wrapper - generates encryption keys if not set

# Generate RSA private key if not set
if [ -z "$RSA_PRIVATE_KEY" ]; then
    echo "🔐 Generating RSA key pair..."
    export RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null)
    echo "✅ RSA key generated"
fi

# Generate data encryption key if not set
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    echo "🔐 Generating data encryption key..."
    export DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
    echo "✅ Data encryption key generated"
fi

# Start the backend
exec /app/nofx
