<h1 align="center">NOFX</h1>

<p align="center">
  <strong>あなた専属の AI トレーディングアシスタント。</strong><br/>
  <strong>あらゆる市場。あらゆるモデル。API キー不要、USDC で支払い。</strong>
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
</p>

<p align="center">
  <a href="../../../README.md">English</a> ·
  <a href="../zh-CN/README.md">中文</a> ·
  <a href="README.md">日本語</a> ·
  <a href="../ko/README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

---

NOFX はオープンソースの**自律型** AI トレーディングアシスタントです。従来の AI ツールのように手動でモデルを設定し、API キーを管理し、データソースを接続する必要はありません — NOFX の AI は**市場を自ら認識し、モデルを自ら選択し、データを自ら取得します**。人間の介入はゼロ。あなたは戦略を設定するだけ、残りは AI が処理します。

**完全自律**: AI がどのモデルを使うか、どの市場データを取得するか、いつ取引するかを自ら判断します。手動のモデル設定不要。複数サービスの API キー管理不要。USDC ウォレットに入金して実行するだけ。

他との違い：**[x402](https://x402.org) マイクロペイメント内蔵**。API キー不要。USDC ウォレットに入金してリクエストごとに支払い。ウォレットがあなたの身分証明。

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

**http://127.0.0.1:3000** を開く。完了。

---

## x402 の仕組み

従来のフロー：アカウント登録 → クレジット購入 → API キー取得 → クォータ管理 → キーのローテーション。

x402 フロー：

```
リクエスト → 402（価格提示）→ ウォレットが USDC を署名 → リトライ → 完了
```

アカウント不要。API キー不要。前払いクレジット不要。ウォレット1つで全モデル。

### 内蔵 x402 プロバイダー

| プロバイダー | チェーン | モデル |
|:---------|:------|:-------|
| <img src="../../../web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4, Claude Opus, DeepSeek, Qwen, Grok, Gemini, Kimi — 15+ モデル |

---

## 機能

| 機能 | 説明 |
|:--------|:------------|
| **マルチ AI** | DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi, MiniMax — いつでも切替 |
| **マルチ取引所** | Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster, Lighter |
| **ストラテジースタジオ** | ビジュアルビルダー — コインソース、インジケーター、リスク管理 |
| **AI ディベートアリーナ** | 複数 AI が取引を議論（ブル vs ベア vs アナリスト）、投票、実行 |
| **AI 競争** | AI がリアルタイムで競争、リーダーボードで成績ランキング |
| **Telegram エージェント** | トレーディングアシスタントとチャット — ストリーミング、ツール呼び出し、メモリ |
| **バックテストラボ** | 過去データシミュレーション、エクイティカーブと成績指標 |
| **ダッシュボード** | ライブポジション、損益、Chain of Thought 付き AI 判断ログ |

### 市場

暗号通貨 · 米国株 · FX · 貴金属

### 取引所 (CEX)

| 取引所 | ステータス | 登録 (手数料割引) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [登録](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [登録](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [登録](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [登録](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [登録](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [登録](https://www.gatenode.xyz/share/VQBGUAxY) |

### 取引所 (Perp-DEX)

| 取引所 | ステータス | 登録 (手数料割引) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [登録](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [登録](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [登録](https://app.lighter.xyz/?referral=68151432) |

### AI モデル (API キーモード)

| AI モデル | ステータス | API キー取得 |
|:---------|:------:|:------------|
| <img src="../../../web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [API キー取得](https://platform.deepseek.com) |
| <img src="../../../web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **Qwen** | ✅ | [API キー取得](https://dashscope.console.aliyun.com) |
| <img src="../../../web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [API キー取得](https://platform.openai.com) |
| <img src="../../../web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [API キー取得](https://console.anthropic.com) |
| <img src="../../../web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [API キー取得](https://aistudio.google.com) |
| <img src="../../../web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [API キー取得](https://console.x.ai) |
| <img src="../../../web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [API キー取得](https://platform.moonshot.cn) |
| <img src="../../../web/public/icons/minimax.svg" width="20" height="20" style="vertical-align: middle;"/> **MiniMax** | ✅ | [API キー取得](https://platform.minimaxi.com) |

### AI モデル (x402 モード — API キー不要)

15+ モデルを [Claw402](https://claw402.ai) 経由で利用 — USDC ウォレットのみ

---

## インストール

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (クラウド)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### ソースから

```bash
# 前提条件: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # バックエンド
cd web && npm install && npm run dev  # フロントエンド（新しいターミナル）
```

---

## リンク

| | |
|:--|:--|
| ウェブサイト | [nofxai.com](https://nofxai.com) |
| ダッシュボード | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API ドキュメント | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **リスク警告**: AI 自動取引には重大なリスクがあります。学習/研究目的または少額でのテストのみを推奨します。

---

## License

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
