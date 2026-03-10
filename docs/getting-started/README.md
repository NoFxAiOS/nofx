# 🚀 Getting Started with NOFX

**Language:** [English](README.md) | [中文](README.zh-CN.md)

This section contains all the documentation you need to get NOFX up and running.

## 📋 Deployment Options

Choose the method that best fits your needs:

### 🐳 Docker Deployment (Recommended)

**Best for:** Beginners, quick setup, production deployments

- **English:** [docker-deploy.en.md](docker-deploy.en.md)
- **中文:** [docker-deploy.zh-CN.md](docker-deploy.zh-CN.md)

**Pros:**
- ✅ One-command setup
- ✅ All dependencies included
- ✅ Easy to update and manage
- ✅ Isolated environment

**Quick Start:**
```bash
cp config.json.example config.json
./scripts/start.sh start --build
```

---


## 🤖 AI Configuration

### Custom AI Providers

- **English:** [custom-api.en.md](custom-api.en.md)
- **中文:** [custom-api.md](custom-api.md)

Use custom AI models or third-party OpenAI-compatible APIs:
- Custom DeepSeek endpoints
- Self-hosted models
- Other LLM providers

---

### 💳 BlockRun Wallet (Pay-per-Request, No API Key)

Access all top AI models by paying with USDC — no API key signup required.

| Provider | Guide | Payment Network |
|----------|-------|-----------------|
| BlockRun (Base Wallet) | [blockrun-base-wallet.md](blockrun-base-wallet.md) | Base (EVM) · USDC |
| BlockRun (Solana Wallet) | [blockrun-sol-wallet.md](blockrun-sol-wallet.md) | Solana · USDC |

**How it works:** Each AI request automatically pays a micro-USDC fee via the [x402 payment protocol](https://blockrun.ai). Your private key signs the payment authorization — no funds leave your wallet until the AI response is delivered.

---

## 🔑 Prerequisites

Before starting, ensure you have:

### For Docker Method:
- ✅ Docker 20.10+
- ✅ Docker Compose V2

### For Manual Method:
- ✅ Go 1.21+
- ✅ Node.js 18+
- ✅ TA-Lib library

---

## 📚 Next Steps

After deployment:

1. **Configure AI Models** → Web interface at http://localhost:3000
2. **Set Up Exchange** → Add Binance/Hyperliquid credentials
3. **Create Traders** → Combine AI models with exchanges
4. **Start Trading** → Monitor performance in dashboard

### 🔐 Optional: Enable Admin Mode (Single-User)

For single-tenant/self-hosted usage, you can enable strict admin-only access:

1) In `config.json` set the 2 fields below:
```jsonc
{
	"admin_mode": true,
  ...
  "jwt_secret": "YOUR_JWT_SCR"
}
```
2) Set environment variables (Docker compose already wired):
- `NOFX_ADMIN_PASSWORD` — admin password (plaintext; hashed on startup)

3) Login at `/login` using the admin password. All non-essential endpoints are blocked to unauthenticated users while admin mode is enabled.

---

## ⚠️ Important Notes

**Before Trading:**
- ⚠️ Test on testnet first
- ⚠️ Start with small amounts
- ⚠️ Understand the risks
- ⚠️ Read [Security Policy](../../SECURITY.md)

**API Keys:**
- 🔑 Never commit API keys to git
- 🔑 Use environment variables
- 🔑 Restrict IP access
- 🔑 Enable 2FA on exchanges

---

## 🆘 Troubleshooting

**Common Issues:**

1. **Docker build fails** → Check Docker version, update to 20.10+
2. **TA-Lib not found** → `brew install ta-lib` (macOS) or `apt-get install libta-lib0-dev` (Ubuntu)
3. **Port 8080 in use** → Change `API_PORT` in .env file
4. **Frontend won't connect** → Check backend is running on port 8080

**Need more help?**
- 📖 [FAQ](../guides/faq.zh-CN.md)
- 💬 [Telegram Community](https://t.me/nofx_dev_community)
- 🐛 [GitHub Issues](https://github.com/NoFxAiOS/nofx/issues)

---

[← Back to Documentation Home](../README.md)
