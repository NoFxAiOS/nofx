# 🤖 PROMPT: Build AI-Powered Trading Platform

## 📋 Project Overview

Build a full-stack AI-powered automated trading platform with multi-exchange support, real-time monitoring, advanced analytics, and secure multi-user authentication.

**Project Name:** [Your Project Name]
**Type:** AI Agentic Trading Operating System
**Architecture:** Microservices with Go backend + React frontend

---

## 🎯 Core Requirements

### 1. **Multi-Exchange Trading Support**
- Support for multiple cryptocurrency exchanges (Binance, OKX, Bybit, Hyperliquid, DEX)
- Unified trader interface for seamless exchange switching
- WebSocket real-time market data streaming
- Support for futures trading (long/short positions with leverage)
- Order types: Market, Limit, Stop-Loss, Take-Profit
- Position management: Open, Close, Partial Close, Update SL/TP

### 2. **AI Decision Engine**
- Integration with multiple AI providers (DeepSeek, OpenAI, Qwen, Claude, etc.)
- Chain-of-Thought (CoT) decision reasoning with full trace logging
- Context-aware trading decisions using:
  - Real-time market data (price, volume, orderbook)
  - Technical indicators (RSI, MACD, Bollinger Bands, MA)
  - Current positions and account balance
  - Historical performance and decision logs
- Configurable decision cycles (15s, 30s, 1m, 5m, etc.)
- Custom system prompts and strategy instructions

### 3. **Advanced Analytics Dashboard**
- **Drawdown Analysis**: Max drawdown, current drawdown, recovery statistics
- **Monte Carlo Simulation**: Risk simulation with configurable parameters
- **Correlation Matrix**: Multi-symbol price correlation analysis
- **Performance Attribution**: P&L breakdown by symbol, strategy, timeframe
- **Order Book Analysis**: Depth chart, imbalance detection, large order alerts
- **Real-time Charts**: Equity curve, position history, trade timeline

### 4. **Security & Encryption**
- End-to-end encryption for sensitive data (API keys, private keys)
- RSA encryption for configuration storage
- SQLite database with field-level encryption using AES-256-GCM
- JWT authentication with secure token handling
- 2FA support (TOTP)
- Password strength validation
- Secure key management (encryption keys stored separately)
- Admin mode with beta code access control

### 5. **Decision Logging System**
- Comprehensive logging of every AI decision cycle:
  - System prompt and input prompt sent to AI
  - AI's Chain-of-Thought reasoning output
  - Decision JSON and parsed actions
  - Account state snapshot (balance, margin, positions)
  - Position snapshots with P&L
  - Execution logs and error messages
  - Timing metrics (AI API latency)
- JSON file storage with cycle numbering
- Historical record retrieval (latest N records, by date)
- Data retention policies with automated cleanup

### 6. **Risk Management**
- Account-level risk controls:
  - Max drawdown limits (stop trading on threshold)
  - Position size limits
  - Leverage constraints
  - Daily loss limits
- Position-level controls:
  - Automatic stop-loss and take-profit
  - Trailing stop support
  - Max positions per symbol
- Real-time risk monitoring with alerts

### 7. **Multi-User System**
- User authentication and authorization
- Per-user AI model configurations
- Per-user exchange API credentials (encrypted)
- Multiple traders per user with different strategies
- Role-based access control (Admin/User)
- Activity logging and audit trails

---

## 🏗️ Technical Architecture

### **Backend Stack**

```yaml
Language: Go 1.25+
Framework: Gin (HTTP router)
Database: SQLite with modernc.org/sqlite (pure Go, no CGO)
Encryption:
  - golang.org/x/crypto (AES-256-GCM, RSA)
  - Custom secure storage layer
WebSocket: gorilla/websocket
Authentication: golang-jwt/jwt (JWT tokens)
Logging:
  - zerolog (structured logging)
  - Custom decision logger
Testing: testify, gomonkey (mocking)
External APIs:
  - Exchange APIs: go-binance, custom clients for OKX/Bybit
  - DEX/Blockchain: go-ethereum, go-hyperliquid
  - AI APIs: HTTP clients with streaming support
Environment: godotenv
```

### **Frontend Stack**

```yaml
Language: TypeScript 5+
Framework: React 18
Build Tool: Vite 6
UI Components:
  - Radix UI (accessible primitives)
  - Tailwind CSS 3 (utility-first styling)
  - Framer Motion (animations)
State Management:
  - SWR (data fetching with caching)
  - Zustand (global state)
Charts: Recharts 2
Icons: Lucide React
Date Handling: date-fns
Testing: Vitest, Testing Library
Linting: ESLint, Prettier
Git Hooks: Husky, lint-staged
```

### **DevOps & Deployment**

```yaml
Containerization: Docker + Docker Compose
Services:
  - nofx-backend: Go API server (port 8080)
  - nofx-frontend: Nginx static server + reverse proxy (port 3000)
Networking: Bridge network between services
Health Checks:
  - Backend: /api/health endpoint
  - Frontend: nginx health check
Volumes:
  - Config files (config.json, config.db)
  - Decision logs storage
  - AI prompt templates
  - RSA encryption keys
Environment Variables:
  - DATA_ENCRYPTION_KEY (database encryption)
  - JWT_SECRET (authentication)
  - AI_MAX_TOKENS (AI response limits)
  - TZ (timezone configuration)
Graceful Shutdown: 30s stop grace period
```

---

## 📂 Project Structure

```
project-root/
├── main.go                      # Application entry point
├── go.mod, go.sum               # Go dependencies
├── docker-compose.yml           # Service orchestration
├── .env.example                 # Environment variables template
├── config.json                  # AI models & exchange configs
├── config.db                    # Encrypted user data (SQLite)
├── beta_codes.txt               # Beta access codes
│
├── docker/
│   ├── Dockerfile.backend       # Go backend container
│   └── Dockerfile.frontend      # React frontend + nginx
│
├── api/                         # HTTP API handlers
│   ├── server.go                # Gin server setup & routes
│   ├── analytics_handler.go     # Analytics endpoints
│   ├── trader_handler.go        # Trader management endpoints
│   ├── auth_handler.go          # Authentication endpoints
│   └── middleware.go            # Auth middleware, CORS, etc.
│
├── trader/                      # Exchange integrations
│   ├── auto_trader.go           # Base trader interface
│   ├── binance_futures.go       # Binance futures implementation
│   ├── okx_futures.go           # OKX futures implementation
│   ├── bybit_futures.go         # Bybit futures implementation
│   ├── hyperliquid_trader.go    # Hyperliquid DEX implementation
│   └── aster_trader.go          # Aster DEX implementation
│
├── manager/
│   └── trader_manager.go        # Trader lifecycle management
│
├── decision/                    # AI decision engine
│   ├── decision_engine.go       # Core decision logic
│   └── ai_client.go             # AI API client (streaming)
│
├── logger/                      # Logging system
│   ├── decision_logger.go       # Decision record persistence
│   ├── logger.go                # Application logger
│   ├── telegram_hook.go         # Telegram notification integration
│   └── telegram_sender.go       # Telegram bot sender
│
├── market/                      # Market data providers
│   ├── monitor.go               # WebSocket kline monitor
│   ├── api_client.go            # REST API market data
│   ├── websocket_client.go      # Generic WebSocket client
│   ├── combined_streams.go      # Multi-symbol stream aggregation
│   └── data.go                  # Data structures & calculations
│
├── analytics/                   # Advanced analytics
│   ├── drawdown.go              # Drawdown calculation & analysis
│   ├── montecarlo.go            # Monte Carlo simulation
│   ├── correlation.go           # Symbol correlation matrix
│   ├── performance.go           # Performance attribution
│   └── orderbook.go             # Order book analysis
│
├── auth/
│   └── auth.go                  # JWT token generation & validation
│
├── crypto/                      # Cryptography layer
│   ├── encryption.go            # AES encryption utilities
│   ├── secure_storage.go        # Encrypted config storage
│   └── crypto.go                # RSA key management
│
├── config/                      # Configuration management
│   └── config.go                # Config loading & validation
│
├── hook/                        # Generic hook system
│   └── hooks.go                 # Extensible hook framework
│
├── pool/                        # Resource pooling
│   └── pool.go                  # Connection pool, worker pool
│
├── bootstrap/                   # Application initialization
│   └── bootstrap.go             # Startup sequence
│
├── prompts/                     # AI prompt templates
│   ├── system_prompt.txt        # Default system prompt
│   └── strategy_examples/       # Strategy prompt examples
│
├── scripts/                     # Utility scripts
│   ├── migrate_encryption.go    # Database migration tool
│   └── ENCRYPTION_README.md     # Encryption guide
│
├── docs/                        # Documentation
│   ├── README.md                # Documentation home
│   ├── getting-started/         # Setup guides
│   ├── guides/                  # Feature guides
│   ├── architecture/            # Architecture docs
│   ├── prompt-guide.md          # AI prompt writing guide
│   └── i18n/                    # Internationalization docs
│
├── web/                         # React frontend
│   ├── package.json             # npm dependencies
│   ├── vite.config.ts           # Vite configuration
│   ├── tsconfig.json            # TypeScript configuration
│   ├── tailwind.config.js       # Tailwind CSS configuration
│   ├── .eslintrc.json           # ESLint rules
│   ├── .prettierrc              # Prettier formatting
│   │
│   ├── public/                  # Static assets
│   │   └── favicon.ico
│   │
│   └── src/
│       ├── App.tsx              # Main application component
│       ├── main.tsx             # React entry point
│       ├── index.css            # Global styles
│       │
│       ├── pages/               # Page components
│       │   ├── Dashboard.tsx    # Main dashboard
│       │   ├── AnalyticsPage.tsx # Analytics dashboard
│       │   ├── LoginPage.tsx    # Authentication page
│       │   ├── SettingsPage.tsx # Settings & configuration
│       │   └── TradersPage.tsx  # Trader management
│       │
│       ├── components/          # Reusable UI components
│       │   ├── ui/              # Base UI components (Radix)
│       │   │   ├── Button.tsx
│       │   │   ├── Card.tsx
│       │   │   ├── Dialog.tsx
│       │   │   ├── Table.tsx
│       │   │   └── ...
│       │   │
│       │   ├── TraderCard.tsx   # Trader status card
│       │   ├── PositionTable.tsx # Position list table
│       │   ├── EquityCurve.tsx  # Equity chart
│       │   ├── MonteCarloSimulation.tsx # Monte Carlo chart
│       │   ├── DrawdownChart.tsx # Drawdown visualization
│       │   ├── CorrelationMatrix.tsx # Correlation heatmap
│       │   └── OrderBookDepth.tsx # Order book chart
│       │
│       ├── contexts/            # React contexts
│       │   ├── AuthContext.tsx  # Authentication state
│       │   └── ThemeContext.tsx # Theme management
│       │
│       ├── hooks/               # Custom React hooks
│       │   ├── useTraders.ts    # Trader data fetching
│       │   ├── useAnalytics.ts  # Analytics data fetching
│       │   └── useWebSocket.ts  # WebSocket connection
│       │
│       ├── lib/                 # Utility libraries
│       │   ├── api.ts           # API client
│       │   ├── utils.ts         # Helper functions
│       │   └── constants.ts     # Constants & enums
│       │
│       └── types/               # TypeScript type definitions
│           ├── trader.ts
│           ├── analytics.ts
│           └── api.ts
│
├── nginx/
│   └── nginx.conf               # Nginx configuration for frontend
│
├── .github/                     # GitHub configuration
│   ├── workflows/               # CI/CD pipelines
│   │   ├── backend-tests.yml
│   │   ├── frontend-tests.yml
│   │   └── docker-build.yml
│   ├── ISSUE_TEMPLATE/
│   └── PULL_REQUEST_TEMPLATE/
│
├── .husky/                      # Git hooks
│   ├── pre-commit               # Lint & format before commit
│   └── pre-push                 # Run tests before push
│
└── README.md                    # Project README
```

---

## 🔧 Key Implementation Details

### **1. Safe Type Assertions (Critical!)**

**ALWAYS** use comma-ok idiom for type assertions to prevent panics:

```go
// ❌ DANGEROUS - Will panic if type is wrong
value := data["key"].(string)

// ✅ SAFE - Handles unexpected types gracefully
value, ok := data["key"].(string)
if !ok {
    log.Printf("Unexpected type for key: %T", data["key"])
    return
}
```

Apply this pattern to:
- JSON parsing from exchange APIs
- WebSocket message handling
- sync.Map value retrieval
- interface{} conversions

### **2. Division by Zero Protection**

**ALWAYS** check denominators before division:

```go
// ❌ DANGEROUS
percentage := (change / total) * 100

// ✅ SAFE
percentage := 0.0
if total > 0 {
    percentage = (change / total) * 100
}
```

Apply to:
- Percentage calculations
- Average calculations
- Ratio calculations
- Price change calculations

### **3. Frontend Error Handling**

**ALWAYS** wrap JSON.parse and API calls in try-catch:

```tsx
// ❌ DANGEROUS
const user = JSON.parse(localStorage.getItem('user'))

// ✅ SAFE
try {
    const userData = localStorage.getItem('user')
    const user = userData ? JSON.parse(userData) : null
} catch (error) {
    console.error('Failed to parse user data:', error)
    localStorage.removeItem('user')
}
```

### **4. Goroutine Leak Prevention**

**ALWAYS** use context for cancellation and cleanup:

```go
// ✅ SAFE - Properly cancelable goroutine
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return // Clean exit
        case <-ticker.C:
            // Do work
        }
    }
}()
```

### **5. Struct Field Validation**

**ALWAYS** verify struct fields exist before accessing:

```go
// ✅ Correct field names matching struct definition
type DecisionRecord struct {
    AccountState AccountSnapshot `json:"account_state"`
    CycleNumber  int             `json:"cycle_number"`
}

// Access with correct field names
balance := record.AccountState.TotalBalance
cycle := record.CycleNumber
```

### **6. Input Validation for Analytics**

**ALWAYS** validate data requirements before processing:

```go
// ✅ Check minimum data points
if len(records) < 2 {
    return gin.H{
        "message": "Insufficient data points, need at least 2 records",
        "data": nil,
    }
}
```

### **7. Secure Key Storage**

**NEVER** store sensitive keys in code or config files:

```go
// ✅ Load from environment variables
encryptionKey := os.Getenv("DATA_ENCRYPTION_KEY")
if encryptionKey == "" {
    log.Fatal("DATA_ENCRYPTION_KEY environment variable required")
}

// ✅ Store encrypted in database
encrypted, err := crypto.EncryptAES(apiKey, masterKey)
```

---

## 🚀 Implementation Steps

### **Phase 1: Core Infrastructure (Week 1-2)**

1. **Project Setup**
   - Initialize Go module with Gin framework
   - Set up React + TypeScript + Vite frontend
   - Configure Docker Compose with backend + frontend services
   - Implement environment variable management
   - Set up git hooks (Husky) for code quality

2. **Database & Encryption Layer**
   - SQLite database initialization
   - AES-256-GCM encryption utilities
   - RSA key pair generation & management
   - Secure storage for API keys and configs
   - Database migration scripts

3. **Authentication System**
   - User registration & login endpoints
   - JWT token generation & validation
   - Auth middleware for protected routes
   - Password hashing with bcrypt
   - 2FA/TOTP support (optional)
   - React AuthContext for frontend state

### **Phase 2: Exchange Integration (Week 3-4)**

1. **Base Trader Interface**
   - Define unified trader interface with methods:
     - GetBalance() - Fetch account balance
     - GetPositions() - Get open positions
     - PlaceOrder() - Execute orders
     - CancelOrder() - Cancel pending orders
     - GetOrderStatus() - Check order status
   - Error handling & retry logic
   - Rate limiting to respect exchange limits

2. **Exchange Implementations**
   - Binance Futures trader
     - REST API client for account/order management
     - WebSocket for real-time position updates
     - Leverage and margin mode configuration
   - Additional exchanges (OKX, Bybit, etc.)
     - Follow same interface pattern
     - Handle exchange-specific quirks (precision, order types)

3. **Market Data System**
   - WebSocket kline monitor for multiple symbols
   - REST API fallback for historical data
   - Data caching with sync.Map
   - Technical indicator calculations (RSI, MACD, etc.)

### **Phase 3: AI Decision Engine (Week 5-6)**

1. **AI Client Integration**
   - HTTP client with streaming support (SSE/Server-Sent Events)
   - Support for multiple AI providers (OpenAI, DeepSeek, etc.)
   - Prompt templating system
   - Response parsing (JSON extraction from AI output)
   - Error handling & fallback strategies

2. **Decision Engine**
   - Context gathering:
     - Current market data (price, volume, indicators)
     - Account state (balance, margin, positions)
     - Historical performance
   - Prompt construction with context injection
   - AI API call with CoT reasoning
   - Decision parsing and validation
   - Action execution with error handling

3. **Decision Logger**
   - JSON file-based logging per cycle
   - Record structure:
     - Timestamp & cycle number
     - System prompt & input prompt
     - AI reasoning trace (CoT)
     - Parsed decisions
     - Account & position snapshots
     - Execution logs & errors
   - Historical record retrieval
   - Automated cleanup of old logs

### **Phase 4: Analytics Dashboard (Week 7-8)**

1. **Backend Analytics Endpoints**
   - Drawdown analysis calculation
   - Monte Carlo simulation engine
   - Correlation matrix computation
   - Performance attribution by symbol/strategy
   - Order book depth analysis

2. **Frontend Charts & Visualizations**
   - Equity curve chart (Recharts)
   - Drawdown visualization with recovery periods
   - Monte Carlo simulation paths & confidence intervals
   - Correlation heatmap
   - Order book depth chart
   - Real-time position table with P&L

3. **Risk Monitoring Dashboard**
   - Real-time risk metrics display
   - Alert system for threshold breaches
   - Historical risk timeline
   - Position size visualization

### **Phase 5: Risk Management (Week 9)**

1. **Account-Level Risk Controls**
   - Max drawdown monitoring & auto-stop
   - Daily loss limits
   - Position size constraints
   - Leverage limits per exchange

2. **Position-Level Controls**
   - Automatic stop-loss placement
   - Take-profit targets
   - Trailing stop implementation
   - Position sizing based on risk parameters

### **Phase 6: Multi-User System (Week 10)**

1. **User Management**
   - User registration with beta codes
   - Per-user AI model configurations
   - Per-user exchange credentials (encrypted)
   - Multiple traders per user

2. **Trader Management**
   - Trader creation with custom strategies
   - Start/stop trader controls
   - Real-time status monitoring
   - Performance tracking per trader

### **Phase 7: Testing & Optimization (Week 11-12)**

1. **Backend Testing**
   - Unit tests for core logic (testify)
   - Mock external APIs (gomonkey)
   - Integration tests for exchange clients
   - Load testing for concurrent traders

2. **Frontend Testing**
   - Component tests (Vitest + Testing Library)
   - E2E tests for critical flows
   - Accessibility testing (Radix UI)

3. **Performance Optimization**
   - Database query optimization
   - WebSocket connection pooling
   - Frontend code splitting & lazy loading
   - API response caching with SWR

4. **Security Audit**
   - Penetration testing
   - Dependency vulnerability scanning
   - Encryption implementation review
   - API rate limiting & abuse prevention

---

## 📝 Configuration Files

### **.env.example**

```bash
# Database Encryption
DATA_ENCRYPTION_KEY=your-32-byte-hex-encryption-key-here

# JWT Authentication
JWT_SECRET=your-jwt-secret-key-here

# Server Configuration
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000
NOFX_TIMEZONE=Asia/Shanghai

# AI Configuration
AI_MAX_TOKENS=4000

# Optional: External Services
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

### **config.json** (AI Models & Exchanges)

```json
{
  "ai_models": [
    {
      "id": "deepseek-chat",
      "name": "DeepSeek Chat",
      "provider": "deepseek",
      "api_key_encrypted": "...",
      "base_url": "https://api.deepseek.com",
      "max_tokens": 4000,
      "temperature": 0.7
    }
  ],
  "exchanges": [
    {
      "id": "binance-futures",
      "name": "Binance Futures",
      "type": "binance",
      "api_key_encrypted": "...",
      "api_secret_encrypted": "...",
      "testnet": false
    }
  ],
  "default_strategy_prompt": "You are an expert crypto trader...",
  "risk_limits": {
    "max_drawdown_percent": 20,
    "max_position_size_usd": 10000,
    "max_leverage": 10
  }
}
```

### **Dockerfile.backend**

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nofx .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl tzdata

WORKDIR /root/

COPY --from=builder /app/nofx .

EXPOSE 8080

CMD ["./nofx"]
```

### **Dockerfile.frontend**

```dockerfile
FROM node:20-alpine AS builder

WORKDIR /app

COPY web/package*.json ./
RUN npm ci

COPY web/ .
RUN npm run build

# Production stage
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### **nginx.conf**

```nginx
server {
    listen 80;
    server_name localhost;

    # Frontend static files
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests to backend
    location /api/ {
        proxy_pass http://nofx:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # WebSocket support
    location /ws/ {
        proxy_pass http://nofx:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }

    # Health check
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
```

---

## 🎨 UI/UX Guidelines

### **Design Principles**

1. **Dark Mode First**: Trading platforms work better in dark mode
2. **Real-time Updates**: Use WebSocket for live data, SWR for polling fallback
3. **Responsive Layout**: Mobile-friendly design with Tailwind breakpoints
4. **Accessible**: Use Radix UI primitives for keyboard navigation & screen readers
5. **Performance**: Lazy load heavy components, virtualize long lists
6. **Animations**: Subtle Framer Motion animations for state changes

### **Color Palette** (Tailwind)

```javascript
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        primary: '#10b981',    // Green for profits
        danger: '#ef4444',     // Red for losses
        warning: '#f59e0b',    // Yellow for warnings
        info: '#3b82f6',       // Blue for info
        background: '#0f172a', // Dark background
        card: '#1e293b',       // Card background
        border: '#334155',     // Border color
      }
    }
  }
}
```

### **Key UI Components**

1. **Trader Card**: Status, P&L, start/stop controls
2. **Position Table**: Symbol, side, size, entry, current, P&L, actions
3. **Equity Curve Chart**: Line chart with time-based zoom
4. **Decision Log Viewer**: Expandable cards with CoT reasoning
5. **Risk Metrics Panel**: Real-time drawdown, leverage, exposure
6. **Alert System**: Toast notifications for important events

---

## 🔒 Security Best Practices

1. **Encryption**
   - Use AES-256-GCM for sensitive data at rest
   - Use TLS/HTTPS for all network communication
   - Rotate encryption keys periodically
   - Store master keys in environment variables, never in code

2. **Authentication**
   - Implement JWT with short expiry (15min access + 7 day refresh)
   - Use httpOnly cookies for token storage (XSS protection)
   - Implement rate limiting on auth endpoints
   - Add CAPTCHA for registration/login (optional)

3. **Input Validation**
   - Validate all user inputs on both frontend & backend
   - Sanitize inputs to prevent SQL injection (use parameterized queries)
   - Validate exchange API responses before processing
   - Set reasonable limits on all numeric inputs

4. **API Security**
   - Use CORS middleware with whitelist
   - Implement rate limiting per user
   - Log all API requests for audit
   - Set request size limits

5. **Secrets Management**
   - Never commit secrets to git (.env in .gitignore)
   - Use different secrets for dev/prod environments
   - Encrypt API keys before storing in database
   - Use read-only volumes for sensitive files in Docker

---

## 📊 Monitoring & Logging

### **Application Logging**

```go
// Use structured logging with zerolog
log.Info().
    Str("trader_id", traderID).
    Str("symbol", symbol).
    Float64("price", price).
    Msg("Order placed successfully")
```

### **Metrics to Track**

1. **Trading Metrics**
   - Total P&L (daily, weekly, monthly)
   - Win rate & profit factor
   - Average trade duration
   - Sharpe ratio & Sortino ratio

2. **System Metrics**
   - AI API latency
   - Exchange API latency
   - Order execution time
   - WebSocket connection stability

3. **Risk Metrics**
   - Current drawdown
   - Max drawdown
   - Leverage utilization
   - Position concentration

### **Alerting**

Implement Telegram notifications for:
- Large losses (> 5% account balance)
- Max drawdown threshold reached
- System errors or crashes
- Unusual AI behavior (invalid decisions)

---

## 🧪 Testing Strategy

### **Backend Tests**

```go
// Unit test example
func TestCalculateDrawdown(t *testing.T) {
    points := []EquityPoint{
        {Equity: 1000, Timestamp: time.Now()},
        {Equity: 900, Timestamp: time.Now()},
        {Equity: 1100, Timestamp: time.Now()},
    }

    result, err := CalculateDrawdown(points)
    assert.NoError(t, err)
    assert.Equal(t, 10.0, result.MaxDrawdown)
}

// Integration test example
func TestBinanceGetBalance(t *testing.T) {
    // Use testnet credentials
    trader := NewBinanceTrader(testnetAPIKey, testnetSecret)
    balance, err := trader.GetBalance()

    assert.NoError(t, err)
    assert.NotNil(t, balance)
}
```

### **Frontend Tests**

```tsx
// Component test example
import { render, screen } from '@testing-library/react'
import { TraderCard } from './TraderCard'

test('renders trader status correctly', () => {
  render(<TraderCard status="running" pnl={150.50} />)

  expect(screen.getByText(/running/i)).toBeInTheDocument()
  expect(screen.getByText(/150.50/i)).toBeInTheDocument()
})
```

---

## 📚 Documentation Requirements

Create comprehensive documentation:

1. **README.md**: Project overview, features, quick start
2. **GETTING_STARTED.md**: Detailed setup instructions
3. **API.md**: API endpoint documentation with examples
4. **PROMPT_GUIDE.md**: Guide for writing effective AI trading prompts
5. **SECURITY.md**: Security policies & vulnerability reporting
6. **CONTRIBUTING.md**: Contribution guidelines
7. **CHANGELOG.md**: Version history & release notes
8. **Architecture Diagrams**: System architecture, data flow, deployment

---

## 🚀 Deployment Checklist

Before going to production:

- [ ] Change all default secrets (JWT_SECRET, DATA_ENCRYPTION_KEY)
- [ ] Enable HTTPS with valid SSL certificates
- [ ] Set up automated backups for database
- [ ] Configure log rotation (avoid disk space issues)
- [ ] Set up monitoring & alerting (Telegram, email)
- [ ] Implement rate limiting on all endpoints
- [ ] Test graceful shutdown & recovery
- [ ] Run security audit & penetration testing
- [ ] Load test with expected user count
- [ ] Prepare incident response plan
- [ ] Set up CI/CD pipeline for automated deployment
- [ ] Document deployment process & rollback procedures

---

## ⚠️ Important Disclaimers

Add these disclaimers prominently:

1. **Risk Warning**: Automated trading carries substantial risk of loss. Only trade with capital you can afford to lose.

2. **No Financial Advice**: This software is for educational/research purposes. It does not constitute financial advice.

3. **No Warranty**: The software is provided "as-is" without warranty of any kind.

4. **Exchange API Risks**: Exchange APIs may have downtime or bugs. Always monitor your positions manually.

5. **AI Limitations**: AI models can make mistakes or generate invalid decisions. Implement safeguards and limits.

---

## 📦 Deliverables Summary

Use this prompt to build:

✅ Full-stack trading platform (Go + React)
✅ Multi-exchange support with unified interface
✅ AI decision engine with multiple providers
✅ Advanced analytics dashboard
✅ Secure multi-user authentication
✅ Comprehensive risk management
✅ Decision logging & audit trails
✅ Real-time monitoring & alerts
✅ Docker deployment setup
✅ Complete documentation

---

## 🎯 Next Steps

1. **Copy this entire prompt** to Claude Code or your AI assistant
2. **Customize** project name, color scheme, and specific requirements
3. **Start with Phase 1** (Core Infrastructure)
4. **Iterate** through each phase systematically
5. **Test thoroughly** at each stage
6. **Deploy** to production with proper security measures

Good luck building your AI trading platform! 🚀

---

**Generated from NOFX Project Analysis**
This prompt contains best practices and lessons learned from building a production AI trading platform.
