# 🔧 Troubleshooting Guide

This guide helps you diagnose and fix common issues before submitting a bug report.

---

## 📋 Quick Diagnostic Checklist

Before reporting a bug, please check:

1. ✅ **Backend is running**: `docker compose ps` or `ps aux | grep nofx`
2. ✅ **Frontend is accessible**: Open http://localhost:3000 in browser
3. ✅ **API is responding**: `curl http://localhost:8080/api/health`
4. ✅ **Check logs for errors**: See [How to Capture Logs](#how-to-capture-logs) below

---

## 🐛 Common Issues & Solutions

### 1. Trading Issues

#### ❌ Only Opening Short Positions (Issue #202)

**Symptom:** AI only opens short positions, never long positions, even when market is bullish.

**Root Cause:** Binance account is in **One-way Mode** instead of **Hedge Mode**.

**Solution:**
1. Login to [Binance Futures](https://www.binance.com/futures/BTCUSDT)
2. Click **⚙️ Preferences** (top right)
3. Select **Position Mode**
4. Switch to **Hedge Mode** (双向持仓)
5. ⚠️ **Important:** Close all positions before switching

**Why this happens:**
- Code uses `PositionSide(LONG)` and `PositionSide(SHORT)` parameters
- These only work in Hedge Mode
- In One-way Mode, orders fail or only one direction works

**For Subaccounts:**
- Some Binance subaccounts may not have permission to change position mode
- Use main account or contact Binance support to enable this permission

---

#### ❌ Order Error: `code=-4061` Position Side Mismatch

**Error Message:** `Order's position side does not match user's setting`

**Solution:** Same as above - switch to Hedge Mode.

---

#### ❌ Leverage Error: `Subaccounts restricted to 5x leverage`

**Symptom:** Orders fail with leverage error when trying to use >5x leverage.

**Solution:**
1. Open Web UI → Trader Settings
2. Set leverage to 5x or lower:
   ```json
   {
     "btc_eth_leverage": 5,
     "altcoin_leverage": 5
   }
   ```
3. Or use main account (supports up to 50x BTC/ETH, 20x altcoins)

---

#### ❌ Positions Not Executing

**Check these:**
1. **API Permissions**:
   - Go to Binance → API Management
   - Verify "Enable Futures" is checked
   - Check IP whitelist (if enabled)

2. **Account Balance**:
   - Ensure sufficient USDT in Futures wallet
   - Check margin usage is not at 100%

3. **Symbol Status**:
   - Verify trading pair is active on exchange
   - Check if symbol is in maintenance mode

4. **Decision Logs**:
   ```bash
   # Check latest decision
   ls -lt decision_logs/your_trader_id/ | head -5
   cat decision_logs/your_trader_id/latest_file.json
   ```
   - Look for AI decision: was it "wait", "hold", or actual trade?
   - Check if position_size_usd is within limits

---

### 2. AI Decision Issues

#### ❌ AI Always Says "Wait" / "Hold"

**Possible Causes:**
1. **Market Conditions**: AI may genuinely see no good opportunities
2. **Risk Limits**: Account equity too low, margin usage too high
3. **Historical Performance**: AI being cautious after losses

**How to Check:**
```bash
# View latest decision reasoning
cat decision_logs/your_trader_id/$(ls -t decision_logs/your_trader_id/ | head -1)
```

Look at the AI's Chain-of-Thought reasoning section.

**Solutions:**
- Wait for better market conditions
- Check if all candidate coins have low liquidity
- Verify `use_default_coins: true` or coin pool API is working

---

#### ❌ AI Making Bad Decisions

**Remember:** AI trading is experimental and not guaranteed to be profitable.

**Things to Check:**
1. **Decision Interval**: Is it too short? (Recommended: 3-5 minutes)
2. **Leverage Settings**: Too aggressive?
3. **Historical Feedback**: Check performance logs to see if AI is learning
4. **Market Volatility**: High volatility = higher risk

**Adjustments:**
- Reduce leverage for more conservative trading
- Increase decision interval to reduce over-trading
- Use smaller initial balance for testing

---

### 3. Connection & API Issues

#### ❌ Docker Image Pull Failed (China Mainland)

**Error:** `ERROR [internal] load metadata for docker.io/library/...`

**Symptoms:**
- `docker compose build` or `docker compose up` hangs
- Timeout errors: `timeout`, `connection refused`
- Cannot pull images from Docker Hub

**Root Cause:**
Access to Docker Hub is restricted or extremely slow in mainland China.

**Solution 1: Configure Docker Registry Mirror (Recommended)**

1. **Edit Docker configuration file:**
   ```bash
   # Linux
   sudo nano /etc/docker/daemon.json

   # macOS (Docker Desktop)
   # Settings → Docker Engine
   ```

2. **Add China registry mirrors:**
   ```json
   {
     "registry-mirrors": [
       "https://docker.m.daocloud.io",
       "https://docker.1panel.live",
       "https://hub.rat.dev",
       "https://dockerpull.com",
       "https://dockerhub.icu"
     ]
   }
   ```

3. **Restart Docker:**
   ```bash
   # Linux
   sudo systemctl restart docker

   # macOS/Windows
   # Restart Docker Desktop
   ```

4. **Rebuild:**
   ```bash
   docker compose build --no-cache
   docker compose up -d
   ```

**Solution 2: Use VPN**

1. Connect to VPN (Taiwan nodes recommended)
2. Ensure **global mode** instead of rule-based mode
3. Re-run `docker compose build`

**Solution 3: Offline Image Download**

If above methods don't work:

1. **Use image proxy websites:**
   - https://proxy.vvvv.ee/images.html (offline download available)
   - https://github.com/dongyubin/DockerHub (mirror list)

2. **Manually import images:**
   ```bash
   # After downloading image files
   docker load -i golang-1.25-alpine.tar
   docker load -i node-20-alpine.tar
   docker load -i nginx-alpine.tar
   ```

3. **Verify images are loaded:**
   ```bash
   docker images | grep golang
   docker images | grep node
   docker images | grep nginx
   ```

**Verify registry mirror is working:**
```bash
# Check Docker info
docker info | grep -A 10 "Registry Mirrors"

# Should show your configured mirrors
```

**Related Issue:** [#168](https://github.com/tinkle-community/nofx/issues/168)

---

#### ❌ Backend Won't Start

**Error:** `port 8080 already in use`

**Solution:**
```bash
# Find what's using the port
lsof -i :8080
# OR
netstat -tulpn | grep 8080

# Kill the process or change port in .env
NOFX_BACKEND_PORT=8081
```

---

#### ❌ Frontend Can't Connect to Backend

**Symptoms:**
- UI shows "Loading..." forever
- Browser console shows 404 or network errors

**Solutions:**
1. **Check backend is running:**
   ```bash
   docker compose ps  # Should show backend as "Up"
   # OR
   curl http://localhost:8080/api/health  # Should return {"status":"ok"}
   ```

2. **Check port configuration:**
   - Backend default: 8080
   - Frontend default: 3000
   - Verify `.env` settings match

3. **CORS Issues:**
   - If running frontend and backend on different ports/domains
   - Check browser console for CORS errors
   - Backend should allow frontend origin

---

#### ❌ Exchange API Errors

**Common Errors:**
- `code=-1021, msg=Timestamp for this request is outside of the recvWindow`
- `invalid signature`
- `timestamp` errors

**Root Cause:**
System time is inaccurate, differing from Binance server time by more than allowed range (typically 5 seconds).

**Solution 1: Sync System Time (Recommended)**

```bash
# Method 1: Use ntpdate (most common)
sudo ntpdate pool.ntp.org

# Method 2: Use other NTP servers
sudo ntpdate -s time.nist.gov
sudo ntpdate -s ntp.aliyun.com  # Aliyun NTP (fast in China)

# Method 3: Enable automatic time sync (Linux)
sudo timedatectl set-ntp true

# Verify time is correct
date
# Should show current accurate time
```

**Docker Environment Special Note:**

If using Docker, container time may be out of sync with host:

```bash
# Check container time
docker exec nofx-backend date

# If time is wrong, restart Docker service
sudo systemctl restart docker

# Or add timezone in docker-compose.yml
environment:
  - TZ=Asia/Shanghai  # or your timezone
```

**Solution 2: Verify API Keys**

If errors persist after time sync:

1. **Check API Keys:**
   - Not expired
   - Have correct permissions (Futures enabled)
   - IP whitelist includes your server IP

2. **Regenerate API Keys:**
   - Login to Binance → API Management
   - Delete old key
   - Create new key
   - Update NOFX configuration

**Solution 3: Check Rate Limits**

Binance has strict API rate limits:

- **Requests per minute limit**
- Reduce number of traders
- Increase decision interval (e.g., from 1min to 3-5min)

**Related Issue:** [#60](https://github.com/tinkle-community/nofx/issues/60)

---

### 4. Frontend Issues

#### ❌ UI Not Updating / Showing Old Data

**Solutions:**
1. **Hard Refresh:**
   - Chrome/Firefox: `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
   - Safari: `Cmd+Option+R`

2. **Clear Browser Cache:**
   - Settings → Privacy → Clear browsing data
   - Or open in Incognito/Private mode

3. **Check SWR Polling:**
   - Frontend uses SWR with 5-10s intervals
   - Data should auto-refresh
   - Check browser console for fetch errors

---

#### ❌ Charts Not Rendering

**Possible Causes:**
1. No historical data yet (system just started)
2. JavaScript errors in console
3. Browser compatibility issues

**Solutions:**
- Wait 5-10 minutes for data to accumulate
- Check browser console (F12) for errors
- Try different browser (Chrome recommended)
- Ensure backend API endpoints are returning data

---

### 5. Database Issues

#### ❌ `database is locked` Error

**Cause:** SQLite database being accessed by multiple processes.

**Solution:**
```bash
# Stop all NOFX processes
docker compose down
# OR
pkill nofx

# Restart
docker compose up -d
# OR
./nofx
```

---

#### ❌ Trader Configuration Not Saving

**Check:**
1. **Permissions:**
   ```bash
   ls -l config.db trading.db
   # Should be writable by current user
   ```

2. **Disk Space:**
   ```bash
   df -h  # Ensure disk not full
   ```

3. **Database Integrity:**
   ```bash
   sqlite3 config.db "PRAGMA integrity_check;"
   ```

---

## 📊 How to Capture Logs

### Backend Logs

**Docker:**
```bash
# View last 100 lines
docker compose logs backend --tail=100

# Follow live logs
docker compose logs -f backend

# Save to file
docker compose logs backend --tail=500 > backend_logs.txt
```

**Manual/PM2:**
```bash
# Terminal where you ran ./nofx shows logs

# PM2:
pm2 logs nofx --lines 100

# Save to file
pm2 logs nofx --lines 500 > backend_logs.txt
```

---

### Frontend Logs (Browser Console)

1. **Open DevTools:**
   - Press `F12` or Right-click → Inspect

2. **Console Tab:**
   - See JavaScript errors and warnings
   - Look for red error messages

3. **Network Tab:**
   - Filter by "XHR" or "Fetch"
   - Look for failed requests (red status codes)
   - Click on failed request → Preview/Response to see error details

4. **Capture Screenshot:**
   - Windows: `Win+Shift+S`
   - Mac: `Cmd+Shift+4`
   - Or use browser DevTools screenshot feature

---

### Decision Logs (Trading Issues)

```bash
# List recent decision logs
ls -lt decision_logs/your_trader_id/ | head -10

# View latest decision
cat decision_logs/your_trader_id/$(ls -t decision_logs/your_trader_id/ | head -1) | jq .

# Search for specific symbol
grep -r "BTCUSDT" decision_logs/your_trader_id/

# Find decisions that resulted in trades
grep -r '"action": "open_' decision_logs/your_trader_id/
```

**What to look for in decision logs:**
- `chain_of_thought`: AI's reasoning process
- `user_prompt`: Market data AI received
- `decision`: Final decision (action, symbol, leverage, etc.)
- `account_state`: Account balance, margin, positions at decision time
- `execution_result`: Whether trade succeeded or failed

---

## 🔍 Diagnostic Commands

### System Health Check

```bash
# Backend health
curl http://localhost:8080/api/health

# List all traders
curl http://localhost:8080/api/traders

# Check specific trader status
curl http://localhost:8080/api/status?trader_id=your_trader_id

# Get account info
curl http://localhost:8080/api/account?trader_id=your_trader_id
```

### Docker Status

```bash
# Check all containers
docker compose ps

# Check resource usage
docker stats

# Restart specific service
docker compose restart backend
docker compose restart frontend
```

### Database Queries

```bash
# Check traders in database
sqlite3 config.db "SELECT id, name, ai_model_id, exchange_id, is_running FROM traders;"

# Check AI models
sqlite3 config.db "SELECT id, name, model_type, enabled FROM ai_models;"

# Check system config
sqlite3 config.db "SELECT key, value FROM system_config;"
```

---

## 📝 Still Having Issues?

If you've tried all the above and still have problems:

1. **Gather Information:**
   - Backend logs (last 100 lines)
   - Frontend console screenshot
   - Decision logs (if trading issue)
   - Your environment details

2. **Submit Bug Report:**
   - Use the [Bug Report Template](../../.github/ISSUE_TEMPLATE/bug_report.md)
   - Include all logs and screenshots
   - Describe what you've already tried

3. **Join Community:**
   - [Telegram Developer Community](https://t.me/nofx_dev_community)
   - [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions)

---

## 🆘 Emergency: System Completely Broken

**Complete Reset (⚠️ Will lose trading history):**

```bash
# Stop everything
docker compose down

# Backup databases (just in case)
cp config.db config.db.backup
cp trading.db trading.db.backup

# Remove databases (fresh start)
rm config.db trading.db

# Restart
docker compose up -d --build

# Reconfigure through web UI
open http://localhost:3000
```

**Partial Reset (Keep configuration, clear logs):**

```bash
# Clear decision logs
rm -rf decision_logs/*

# Clear Docker cache and rebuild
docker compose down
docker compose build --no-cache
docker compose up -d
```

---

## 📚 Additional Resources

- **[FAQ](faq.en.md)** - Frequently Asked Questions
- **[Getting Started](../getting-started/README.md)** - Setup guide
- **[Architecture Docs](../architecture/README.md)** - How the system works
- **[CLAUDE.md](../../CLAUDE.md)** - Developer documentation

---

**Last Updated:** 2025-11-02
