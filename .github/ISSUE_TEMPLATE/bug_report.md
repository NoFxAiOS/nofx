---
name: Bug Report
about: Report a bug to help us improve NOFX
title: '[BUG] '
labels: bug
assignees: ''
---

> **⚠️ Before submitting:** Please check the [Troubleshooting Guide](../../docs/guides/TROUBLESHOOTING.md) ([中文版](../../docs/guides/TROUBLESHOOTING.zh-CN.md)) to see if your issue can be resolved quickly.

## 🐛 Bug Description
<!-- A clear and concise description of what the bug is -->


## 🔍 Bug Category
<!-- Check the category that best describes this bug -->
- [ ] Trading execution (orders not executing, wrong position size, etc.)
- [ ] AI decision issues (unexpected decisions, only opening one direction, etc.)
- [ ] Exchange connection (API errors, authentication failures, etc.)
- [ ] UI/Frontend (display issues, buttons not working, data not updating, etc.)
- [ ] Backend/API (server errors, crashes, performance issues, etc.)
- [ ] Configuration (settings not saving, database errors, etc.)
- [ ] Other: _________________

## 📋 Steps to Reproduce
1. Go to '...'
2. Click on '...' / Run command '...'
3. Configure '...'
4. See error

## ✅ Expected Behavior
<!-- What you expected to happen -->


## ❌ Actual Behavior
<!-- What actually happened -->


## 📸 Screenshots & Logs

### Frontend Error (if applicable)
<!-- How to capture frontend errors: -->
<!-- 1. Open browser DevTools (F12 or Right-click → Inspect) -->
<!-- 2. Go to "Console" tab to see JavaScript errors -->
<!-- 3. Screenshot the error messages -->
<!-- 4. Check "Network" tab for failed API requests (show status code & response) -->

**Browser Console Screenshot:**
<!-- Paste screenshot here -->

**Network Tab (failed requests):**
<!-- Paste screenshot of failed API calls here -->

### Backend Logs (if applicable)
<!-- How to capture backend logs: -->

**Docker users:**
```bash
# View backend logs
docker compose logs backend --tail=100

# OR continuously follow logs
docker compose logs -f backend
```

**Manual/PM2 users:**
```bash
# Terminal output where you ran: ./nofx
# OR PM2 logs:
pm2 logs nofx --lines 100
```

**Backend Log Output:**
```
Paste backend logs here (last 50-100 lines around the error)
```

### Trading/Decision Logs (if trading issue)
<!-- Decision logs are saved in: decision_logs/{trader_id}/ -->
<!-- Find the latest JSON file and paste relevant parts -->

**Decision Log Path:** `decision_logs/{trader_id}/{timestamp}.json`

```json
{
  "paste relevant decision log here if applicable"
}
```

## 💻 Environment

**System:**
- **OS:** [e.g. macOS 13, Ubuntu 22.04, Windows 11]
- **Deployment:** [Docker / Manual / PM2]

**Backend:**
- **Go Version:** [run: `go version`]
- **NOFX Version:** [run: `git log -1 --oneline` or check release tag]

**Frontend:**
- **Browser:** [e.g. Chrome 120, Firefox 121, Safari 17]
- **Node.js Version:** [run: `node -v`]

**Trading Setup:**
- **Exchange:** [Binance / Hyperliquid / Aster]
- **Account Type:** [Main Account / Subaccount]
- **Position Mode:** [Hedge Mode (Dual) / One-way Mode] ← **Important for trading bugs!**
- **AI Model:** [DeepSeek / Qwen / Custom]
- **Number of Traders:** [e.g. 1, 2, etc.]

## 🔧 Configuration (if relevant)
<!-- Only include non-sensitive parts of your config -->
<!-- ⚠️ NEVER paste API keys or private keys! -->

**Leverage Settings:**
```json
{
  "btc_eth_leverage": 5,
  "altcoin_leverage": 5
}
```

**Any custom settings:**
<!-- e.g. modified scan_interval, custom coin list, etc. -->


## 📊 Additional Context

**Frequency:**
- [ ] Happens every time
- [ ] Happens randomly
- [ ] Happened once

**Timeline:**
- Did this work before? [ ] Yes [ ] No
- When did it break? [e.g. after upgrade to v3.0.0, after changing config, etc.]
- Recent changes? [e.g. updated dependencies, changed exchange, etc.]

**Impact:**
- [ ] System cannot start
- [ ] Trading stopped/broken
- [ ] UI broken but trading works
- [ ] Minor visual issue
- [ ] Other: _________________

## 💡 Possible Solution
<!-- Optional: If you have ideas on how to fix this, or workarounds you've tried -->


---

## 📝 Quick Tips for Faster Resolution

**For Trading Issues:**
1. ✅ Check Binance position mode: Go to Futures → ⚙️ Preferences → Position Mode → Must be **Hedge Mode**
2. ✅ Verify API permissions: Futures trading must be enabled
3. ✅ Check decision logs in `decision_logs/{trader_id}/` for AI reasoning

**For Connection Issues:**
4. ✅ Test API connectivity: `curl http://localhost:8080/api/health`
5. ✅ Check API rate limits on exchange
6. ✅ Verify API keys are not expired

**For UI Issues:**
7. ✅ Hard refresh: Ctrl+Shift+R (or Cmd+Shift+R on Mac)
8. ✅ Check browser console (F12) for errors
9. ✅ Verify backend is running: `docker compose ps` or `ps aux | grep nofx`
