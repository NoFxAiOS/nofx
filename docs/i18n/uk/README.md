<h1 align="center">NOFX</h1>

<p align="center">
  <strong>Ваш персональний AI торговий асистент.</strong><br/>
  <strong>Будь-який ринок. Будь-яка модель. Оплата USDC, без API ключів.</strong>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=for-the-badge" alt="Stars"></a>
  <a href="https://github.com/NoFxAiOS/nofx/releases"><img src="https://img.shields.io/github/v/release/NoFxAiOS/nofx?style=for-the-badge" alt="Release"></a>
  <a href="https://github.com/NoFxAiOS/nofx/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg?style=for-the-badge" alt="License"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/Telegram-Community-blue?style=for-the-badge&logo=telegram" alt="Telegram"></a>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go"></a>
  <a href="https://reactjs.org/"><img src="https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react" alt="React"></a>
  <a href="https://x402.org"><img src="https://img.shields.io/badge/x402-USDC%20Payments-2775CA?style=flat" alt="x402"></a>
  <a href="https://claw402.ai"><img src="https://img.shields.io/badge/Claw402-AI%20Gateway-FF6B35?style=flat" alt="Claw402"></a>
  <a href="https://blockrun.ai"><img src="https://img.shields.io/badge/BlockRun-x402%20Provider-8B5CF6?style=flat" alt="BlockRun"></a>
</p>

<p align="center">
  <a href="../../../README.md">English</a> ·
  <a href="../zh-CN/README.md">中文</a> ·
  <a href="../ja/README.md">日本語</a> ·
  <a href="../ko/README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

---

NOFX — це **автономний** AI торговий асистент з відкритим кодом. На відміну від традиційних AI інструментів, де потрібно вручну налаштовувати моделі, керувати API ключами та підключати джерела даних — AI у NOFX **сам аналізує ринки, обирає моделі та отримує дані**. Нульове втручання людини. Ви задаєте стратегію, AI робить все інше.

**Повна автономність**: AI сам вирішує, яку модель використовувати, які ринкові дані отримати, коли торгувати. Без ручного налаштування моделей. Без жонглювання API ключами різних сервісів. Просто поповніть USDC гаманець і запустіть.

Ключова відмінність: **вбудовані [x402](https://x402.org) мікроплатежі**. Без API ключів. Поповніть USDC гаманець і платіть за кожен запит. Гаманець — це ваша ідентифікація.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Відкрийте **http://127.0.0.1:3000**. Готово.

---

## Як працює x402

Традиційний процес: реєстрація → купівля кредитів → отримання API ключа → управління квотою → ротація ключів.

x402 процес:

```
Запит → 402 (ось ціна) → гаманець підписує USDC → повтор → готово
```

Без акаунтів. Без API ключів. Без передоплати. Один гаманець, усі моделі.

### Вбудовані x402 провайдери

| Провайдер | Мережа | Моделі |
|:---------|:------|:-------|
| <img src="../../../web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4, Claude Opus, DeepSeek, Qwen, Grok, Gemini, Kimi — 15+ моделей |
| **[BlockRun](https://blockrun.ai)** | Base | Налаштовуваний |
| **[BlockRun Sol](https://sol.blockrun.ai)** | Solana | Налаштовуваний |

Сумісний з **[ClawRouter](https://github.com/BlockRunAI/ClawRouter)** — інтелектуальний LLM маршрутизатор (41+ моделей, економія 74-100%, <1ms маршрутизація).

---

## Можливості

| Функція | Опис |
|:--------|:------------|
| **Мульти-AI** | DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi — перемикання будь-коли |
| **Мульти-біржа** | Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster, Lighter |
| **Студія стратегій** | Візуальний конструктор — джерела монет, індикатори, контроль ризиків |
| **AI Арена дебатів** | Кілька AI обговорюють угоди, голосують, виконують |
| **AI Змагання** | AI змагаються в реальному часі, рейтинг у таблиці лідерів |
| **Telegram Агент** | Чат з торговим асистентом — стрімінг, виклик інструментів, пам'ять |
| **Лабораторія бектесту** | Історична симуляція з кривою капіталу та метриками |
| **Панель управління** | Позиції в реальному часі, P/L, логи AI рішень з Chain of Thought |

### Ринки

Криптовалюта · Акції США · Форекс · Метали

### Біржі (CEX)

| Біржа | Статус | Реєстрація (знижка) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [Реєстрація](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [Реєстрація](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [Реєстрація](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [Реєстрація](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [Реєстрація](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [Реєстрація](https://www.gatenode.xyz/share/VQBGUAxY) |

### Біржі (Perp-DEX)

| Біржа | Статус | Реєстрація (знижка) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [Реєстрація](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [Реєстрація](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [Реєстрація](https://app.lighter.xyz/?referral=68151432) |

### AI Моделі (Режим API ключів)

| AI Модель | Статус | Отримати API ключ |
|:---------|:------:|:------------|
| <img src="../../../web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [Отримати](https://platform.deepseek.com) |
| <img src="../../../web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **Qwen** | ✅ | [Отримати](https://dashscope.console.aliyun.com) |
| <img src="../../../web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [Отримати](https://platform.openai.com) |
| <img src="../../../web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [Отримати](https://console.anthropic.com) |
| <img src="../../../web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [Отримати](https://aistudio.google.com) |
| <img src="../../../web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [Отримати](https://console.x.ai) |
| <img src="../../../web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [Отримати](https://platform.moonshot.cn) |

### AI Моделі (Режим x402 — без API ключів)

15+ моделей через [Claw402](https://claw402.ai) або [BlockRun](https://blockrun.ai) — лише USDC гаманець

---

## Встановлення

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (Хмара)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### З вихідного коду

```bash
# Вимоги: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib
# Ubuntu: sudo apt-get install libta-lib0-dev

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # бекенд
cd web && npm install && npm run dev  # фронтенд (новий термінал)
```

---

## Посилання

| | |
|:--|:--|
| Сайт | [nofxai.com](https://nofxai.com) |
| Панель | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API Документація | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **Попередження**: AI автоторгівля несе значні ризики. Рекомендується лише для навчання/досліджень або тестування малих сум.

---

## Ліцензія

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
