<h1 align="center">NOFX</h1>

<p align="center">
  <strong>Ваш персональный AI торговый ассистент.</strong><br/>
  <strong>Любой рынок. Любая модель. Оплата USDC, без API ключей.</strong>
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
  <a href="README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

---

NOFX — это **автономный** AI торговый ассистент с открытым исходным кодом. В отличие от традиционных AI инструментов, где нужно вручную настраивать модели, управлять API ключами и подключать источники данных — AI в NOFX **сам анализирует рынки, выбирает модели и получает данные**. Нулевое вмешательство человека. Вы задаёте стратегию, AI делает всё остальное.

**Полная автономность**: AI сам решает, какую модель использовать, какие рыночные данные получить, когда торговать. Без ручной настройки моделей. Без жонглирования API ключами разных сервисов. Просто пополните USDC кошелёк и запустите.

Ключевое отличие: **встроенные [x402](https://x402.org) микроплатежи**. Без API ключей. Пополните USDC кошелёк и платите за каждый запрос. Кошелёк — это ваша идентификация.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Откройте **http://127.0.0.1:3000**. Готово.

---

## Как работает x402

Традиционный процесс: регистрация → покупка кредитов → получение API ключа → управление квотой → ротация ключей.

x402 процесс:

```
Запрос → 402 (вот цена) → кошелёк подписывает USDC → повтор → готово
```

Без аккаунтов. Без API ключей. Без предоплаты. Один кошелёк, все модели.

### Встроенные x402 провайдеры

| Провайдер | Сеть | Модели |
|:---------|:------|:-------|
| <img src="../../../web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4, Claude Opus, DeepSeek, Qwen, Grok, Gemini, Kimi — 15+ моделей |
| **[BlockRun](https://blockrun.ai)** | Base | Настраиваемый |
| **[BlockRun Sol](https://sol.blockrun.ai)** | Solana | Настраиваемый |

Совместим с **[ClawRouter](https://github.com/BlockRunAI/ClawRouter)** — интеллектуальный LLM маршрутизатор, автоматически выбирающий самую дешёвую модель (41+ моделей, экономия 74-100%, <1ms маршрутизация).

---

## Возможности

| Функция | Описание |
|:--------|:------------|
| **Мульти-AI** | DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi — переключение в любой момент |
| **Мульти-биржа** | Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster, Lighter |
| **Студия стратегий** | Визуальный конструктор — источники монет, индикаторы, контроль рисков |
| **AI Арена дебатов** | Несколько AI обсуждают сделки (Бык vs Медведь vs Аналитик), голосуют, исполняют |
| **AI Соревнование** | AI соревнуются в реальном времени, рейтинг в таблице лидеров |
| **Telegram Агент** | Чат с торговым ассистентом — стриминг, вызов инструментов, память |
| **Лаборатория бэктеста** | Историческая симуляция с кривой капитала и метриками |
| **Панель управления** | Позиции в реальном времени, P/L, логи AI решений с Chain of Thought |

### Рынки

Криптовалюта · Акции США · Форекс · Металлы

### Биржи (CEX)

| Биржа | Статус | Регистрация (скидка) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [Регистрация](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [Регистрация](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [Регистрация](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [Регистрация](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [Регистрация](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [Регистрация](https://www.gatenode.xyz/share/VQBGUAxY) |

### Биржи (Perp-DEX)

| Биржа | Статус | Регистрация (скидка) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [Регистрация](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [Регистрация](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [Регистрация](https://app.lighter.xyz/?referral=68151432) |

### AI Модели (Режим API ключей)

| AI Модель | Статус | Получить API ключ |
|:---------|:------:|:------------|
| <img src="../../../web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [Получить](https://platform.deepseek.com) |
| <img src="../../../web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **Qwen** | ✅ | [Получить](https://dashscope.console.aliyun.com) |
| <img src="../../../web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [Получить](https://platform.openai.com) |
| <img src="../../../web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [Получить](https://console.anthropic.com) |
| <img src="../../../web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [Получить](https://aistudio.google.com) |
| <img src="../../../web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [Получить](https://console.x.ai) |
| <img src="../../../web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [Получить](https://platform.moonshot.cn) |

### AI Модели (Режим x402 — без API ключей)

15+ моделей через [Claw402](https://claw402.ai) или [BlockRun](https://blockrun.ai) — только USDC кошелёк

---

## Установка

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (Облако)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### Из исходников

```bash
# Требования: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib
# Ubuntu: sudo apt-get install libta-lib0-dev

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # бэкенд
cd web && npm install && npm run dev  # фронтенд (новый терминал)
```

---

## Ссылки

| | |
|:--|:--|
| Сайт | [nofxai.com](https://nofxai.com) |
| Панель | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API Документация | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **Предупреждение**: AI автоторговля несёт значительные риски. Рекомендуется только для обучения/исследований или тестирования малых сумм.

---

## Лицензия

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
