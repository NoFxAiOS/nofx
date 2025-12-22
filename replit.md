# Monnaire Trading Agent OS AI Trading System - Replit Deployment

### Overview
Monnaire Trading Agent OS is an AI-powered cryptocurrency trading system designed for multi-agent AI trading. It supports various AI models (DeepSeek, Qwen) and integrates with multiple cryptocurrency exchanges (OKX, Hyperliquid, Aster DEX). The project aims to provide users with a full-stack, self-hosted platform for automated trading with real-time market data and community-driven features.

### User Preferences
I prefer clear, concise explanations and an iterative development approach. Please ask before making major architectural changes or introducing new dependencies. Ensure that any changes maintain backward compatibility where possible. I want the agent to prioritize stability and security in its recommendations and code modifications.

### System Architecture
The application is a full-stack system comprising a Go backend and a React/Vite frontend.

**UI/UX Decisions:**
- The frontend is a React application built with Vite, located in the `web/` directory.
- In production, the built frontend (`web/dist`) is served directly by the Go backend.

**Technical Implementations:**
- **Backend (Go):**
    - Serves a REST API for trader management.
    - Implements WebSocket for real-time crypto market data.
    - Integrates with OKX, Hyperliquid, and Aster DEX exchanges.
    - Supports AI integration with DeepSeek, Qwen, and custom APIs.
    - Automatically uses Replit's `PORT` environment variable in production.
    - Binary name: `nofx-backend` (compiled from source, Go 1.25.0).
- **Frontend (React + Vite):**
    - Developed using `npm` and `Vite 6.x`.
    - Served by the backend in production.
- **Database:**
    - Primary: Neon PostgreSQL cloud (configured via `DATABASE_URL` environment variable).
    - Fallback: SQLite (`config.db`) if `USE_NEON=false`.
    - Uses `withRetry` logic for critical queries to handle cold start issues and transient errors, especially with Neon PostgreSQL.
    - Database stores user accounts, trader configurations, AI model settings, exchange credentials, and trading history.

**Feature Specifications:**
- **AI Trading:** Multi-agent AI trading with DeepSeek and Qwen models.
- **Exchange Support:** Binance Futures, Hyperliquid, Aster DEX.
- **Real-time Data:** WebSocket for live market data.
- **Security:** Admin mode for testing (bypasses auth), JWT-based authentication (secret should be changed), API keys stored in DB.

**System Design Choices:**
- The application uses a monolithic architecture with the Go backend serving both API and static frontend assets.
- Deployment is optimized for Replit's Reserved VM for persistent processes and WebSocket support.
- Configuration is managed via `config.json` and environment variables.
- Robust error handling for database connections, including retry mechanisms and connection pooling.
- Consistent symbol handling across exchanges using internal formats.

### External Dependencies
- **Databases:**
    - Neon PostgreSQL (cloud database)
    - SQLite (local fallback database)
- **AI Models:**
    - DeepSeek
    - Qwen
    - Custom AI APIs
- **Cryptocurrency Exchanges:**
    - OKX
    - Hyperliquid
    - Aster DEX
- **Frontend Libraries:**
    - React
    - Vite (build tool)
- **Backend Libraries:**
    - Go standard library and relevant packages for web servers, database interaction, and WebSocket handling.