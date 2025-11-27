# Monnaire Trading Agent OS AI Trading System - Replit Deployment

## Project Overview
Monnaire Trading Agent OS is an AI-powered cryptocurrency trading system with support for multiple AI models (DeepSeek, Qwen) and exchanges (OKX, Hyperliquid, Aster DEX). This is a full-stack application with a Go backend and React/Vite frontend.

## Recent Changes (November 27, 2025)
- ✅ **Fixed OKX margin insufficient error** - Added margin check + auto position sizing
- ✅ **Margin guardrails** - Max 80% of available balance, auto-adjust oversized positions
- ✅ **Minimum position check** - Reject trades < $10 to prevent OKX errors
- ✅ **Fixed Neon PostgreSQL cold start issues** - Added connection pool config + retry logic + keepalive
- ✅ **Database connection pool** - MaxOpen=10, MaxIdle=5, IdleTime=30s, Lifetime=5m
- ✅ **Retry logic for critical queries** - GetUserByID, GetExchanges, GetAIModels with exponential backoff
- ✅ **Connection keepalive** - Background goroutine pings DB every 5 minutes
- ✅ **Fixed OKX lot size error** - BNB contract requires integer lot size (lotSz=1)
- ✅ **Updated contract specifications** - Accurate ctVal, minSz, lotSz for all contracts
- ✅ **Added ContractSpec struct** - Properly returns all contract parameters from API
- ✅ **Fixed CORS preflight requests** - OPTIONS requests now bypass auth middleware
- ✅ **Fixed panic in GetAccountInfo** - Safe type assertions for all position fields
- ✅ **Added custom recovery middleware** - CORS headers set even on 500 errors
- ✅ **Fixed PostgreSQL UPSERT syntax** - Using EXCLUDED.column instead of $N

## Previous Changes (November 25, 2025)
- ✅ **Migrated database from SQLite to Neon PostgreSQL cloud**
- ✅ **Dual database support with automatic SQL syntax conversion**
- ✅ **Fixed PostgreSQL compatibility (placeholder conversion ? → $1, $2)**
- ✅ **Fixed COALESCE type matching for PostgreSQL BOOLEAN fields**
- ✅ **Admin user creation working with PostgreSQL**
- ✅ **All configuration synced to Neon cloud database**
- ✅ OKX exchange support (Binance unavailable in US region)
- ✅ Frontend runs on port 5000, backend API on port 8080
- ✅ Admin mode enabled by default (no login required for testing)
- ✅ Deployment configured for Replit (Reserved VM)

## Architecture

### Backend (Go)
- **Port**: Uses `PORT` environment variable (defaults to 8080 in dev)
- **Binary**: `nofx-backend` (compiled from source)
- **Database**: Neon PostgreSQL cloud (primary) + SQLite fallback
- **Features**:
  - REST API for trader management
  - WebSocket for real-time crypto market data
  - Support for OKX, Hyperliquid, Aster exchanges
  - AI integration with DeepSeek, Qwen, and custom APIs
  - **Serves built frontend in production**
  - **Automatically uses Replit's PORT variable in production**

### Database Configuration
- **Primary**: Neon PostgreSQL (DATABASE_URL environment variable)
- **Fallback**: SQLite (`config.db`) if USE_NEON=false
- **Environment Variables**:
  - `USE_NEON=true` - Enable Neon PostgreSQL
  - `DATABASE_URL` - Neon connection string
  - `SQLITE_PATH` - SQLite database path (optional)

### Frontend (React + Vite)
- **Development Port**: 5000 (exposed for Replit webview)
- **Production**: Served by backend from `web/dist`
- **Location**: `web/` directory
- **Package Manager**: npm
- **Build Tool**: Vite 6.x

## Project Structure
```
.
├── monnoire-backend          # Compiled Go binary
├── main.go               # Go backend entry point
├── config.json           # System configuration
├── config.db             # SQLite database
├── api/                  # API server code
├── trader/               # Trading logic
├── market/               # Market data handlers
├── web/                  # Frontend application
│   ├── src/             # React source code
│   ├── dist/            # Production build (generated)
│   ├── package.json     # Frontend dependencies
│   └── vite.config.ts   # Vite configuration
└── prompts/             # AI prompt templates
```

## Running the Application

### Development Mode
The application uses a single workflow that starts both services:
- Backend: `./monnoire-backend` (pre-compiled Go binary on port 8080)
- Frontend: `cd web && npm run dev` (Vite dev server on port 5000)

Both services start automatically via the `fullstack-app` workflow.

### Production Deployment
The deployment is configured for **Replit Reserved VM** with:
- **Deployment Type**: Reserved VM (for WebSocket support and long-running processes)
- **Build Command**: **None** (using pre-built binary)
  - Binary compiled locally with Go 1.25.0
  - Replit deployment only has Go 1.24, so we use pre-built binary
  - Binary size: 40MB (includes all dependencies)
- **Run Command**: Starts the backend
  ```bash
  ./monnoire-backend
  ```
- **Port**: Backend automatically uses Replit's `PORT` environment variable
- **Configuration File**: `.replit` contains deployment settings

The backend serves the built frontend from `web/dist` and provides:
- **Health Check**: `GET /` returns `{"status":"ok","service":"Monnaire Trading Agent OS AI Trading System"}`
- **API Endpoints**: All API routes under `/api/*`
- **Static Assets**: Frontend assets served from `/assets/*`
- **SPA Routing**: Non-API routes serve `index.html` for client-side routing

## Configuration

### System Config (`config.json`)
- **admin_mode**: `true` - Admin mode enabled (no login required)
- **beta_mode**: `false` - Beta access disabled
- **api_server_port**: `8080` - Backend API port
- **jwt_secret**: Pre-configured (should be changed in production)
- **leverage**: BTC/ETH 5x, Altcoins 5x
- **default_coins**: BTCUSDT, ETHUSDT, SOLUSDT, etc.

### Environment Variables
API keys for exchanges and AI models are stored in the SQLite database and configured via the web interface. For trading functionality, users need to configure:
- Exchange credentials (Binance API keys, Hyperliquid private key, etc.)
- AI model API keys (DeepSeek, Qwen, or custom API)

## Development Notes

### Rebuilding the Backend
If you modify the Go code, rebuild the backend:
```bash
go build -o monnoire-backend main.go
```

### Rebuilding the Frontend
To rebuild the production frontend:
```bash
cd web && npm run build
```

### Installing Dependencies
- **Frontend**: `cd web && npm install`
- **Backend**: `go mod download`

### Database
The SQLite database (`config.db`) is created automatically on first run. It stores:
- User accounts
- Trader configurations
- AI model settings
- Exchange credentials
- Trading history

## Deployment to Replit

### Backend-Only Deployment (Current Configuration)

The deployment is configured to deploy **backend only** (no frontend build):
1. Click the **Publish** button in Replit
2. Select **Reserved VM** deployment type
3. Review the configuration (already configured)
4. Click **Publish**

The deployment will:
1. Compile the backend (`go build -o monnoire-backend main.go`)
2. Start the backend binary (REST API server)

### Deployment Health Checks
- **Endpoint**: `GET /`
- **Response**: `{"status":"ok","service":"Monnaire Trading Agent OS AI Trading System"}`
- **Timeout**: Default Replit timeout settings

### Testing the Deployed API

After deployment, use the provided test script:
```bash
# 测试本地API
./test-api.sh

# 测试部署的API
./test-api.sh https://your-deployment.repl.co
```

### API Documentation

完整的API文档和前端对接指南：
- **API Documentation**: See `API_DOCUMENTATION.md`
- **Frontend Integration Guide**: See `FRONTEND_INTEGRATION.md`
- **Test Script**: `test-api.sh`

## Security Notes
- Admin mode is enabled for easy testing (bypasses authentication)
- JWT secret should be changed in production
- API keys and secrets are stored in the database (consider using Replit Secrets for production)
- Default admin user: `admin@localhost`

## Key Features
- Multi-agent AI trading with DeepSeek and Qwen
- Support for Binance Futures, Hyperliquid, and Aster DEX
- Real-time market data via WebSocket
- Community-driven competition system
- Self-hosted, full control over trading logic

## Troubleshooting

### Backend not starting
- Check if `monnoire-backend` binary exists: `ls -lh monnoire-backend`
- Rebuild if needed: `go build -o monnoire-backend main.go`
- Check logs in the workflow console

### Frontend not displaying in production
- Ensure frontend is built: `ls web/dist/index.html`
- Rebuild if needed: `cd web && npm run build`
- Check backend logs for static file serving errors

### Deployment health check failing
- Verify root endpoint responds: `curl http://localhost:8080/`
- Should return: `{"status":"ok","service":"Monnaire Trading Agent OS AI Trading System"}`

### Database issues
- Delete `config.db` to reset (will lose all data)
- Check file permissions: `ls -la config.db`

## Next Steps for Production
1. Change JWT secret in `config.json`
2. Configure real exchange API keys via the web interface
3. Add AI model API keys (DeepSeek or Qwen)
4. Set up proper authentication (disable admin mode)
5. Consider using Replit Secrets for sensitive credentials
6. Test with small amounts before live trading

## Links
- GitHub: https://github.com/your-repo/nofx
- Documentation: See `docs/` directory
- Quick Start: See `QUICK_START.md`
