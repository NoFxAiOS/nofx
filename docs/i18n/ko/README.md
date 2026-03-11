<h1 align="center">NOFX</h1>

<p align="center">
  <strong>당신만의 AI 트레이딩 어시스턴트.</strong><br/>
  <strong>모든 시장. 모든 모델. API 키 없이 USDC로 결제.</strong>
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
  <a href="README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

---

NOFX는 오픈소스 **자율형** AI 트레이딩 어시스턴트입니다. 수동으로 모델을 설정하고, API 키를 관리하고, 데이터 소스를 연결해야 하는 기존 AI 도구와 달리 — NOFX의 AI는 **시장을 스스로 인식하고, 모델을 스스로 선택하고, 데이터를 스스로 가져옵니다**. 인간 개입 제로. 전략만 설정하면 나머지는 AI가 처리합니다.

**완전 자율**: AI가 어떤 모델을 사용할지, 어떤 시장 데이터를 가져올지, 언제 거래할지를 스스로 결정합니다. 수동 모델 설정 불필요. 여러 서비스의 API 키 관리 불필요. USDC 지갑에 충전하고 실행하기만 하면 됩니다.

차별점: **[x402](https://x402.org) 마이크로 결제 내장**. API 키 불필요. USDC 지갑에 충전하고 요청마다 결제. 지갑이 곧 신원.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

**http://127.0.0.1:3000** 을 열면 완료.

---

## x402 작동 방식

기존 플로우: 계정 등록 → 크레딧 구매 → API 키 받기 → 쿼터 관리 → 키 교체.

x402 플로우:

```
요청 → 402 (가격 제시) → 지갑이 USDC 서명 → 재시도 → 완료
```

계정 불필요. API 키 불필요. 선불 크레딧 불필요. 지갑 하나로 모든 모델.

### 내장 x402 프로바이더

| 프로바이더 | 체인 | 모델 |
|:---------|:------|:-------|
| <img src="../../../web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4, Claude Opus, DeepSeek, Qwen, Grok, Gemini, Kimi — 15+ 모델 |
| **[BlockRun](https://blockrun.ai)** | Base | 설정 가능 |
| **[BlockRun Sol](https://sol.blockrun.ai)** | Solana | 설정 가능 |

**[ClawRouter](https://github.com/BlockRunAI/ClawRouter)** 호환 — 요청마다 최저가 모델을 자동 선택하는 지능형 LLM 라우터 (41+ 모델, 74-100% 절감, <1ms 라우팅).

---

## 기능

| 기능 | 설명 |
|:--------|:------------|
| **멀티 AI** | DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi — 언제든 전환 |
| **멀티 거래소** | Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster, Lighter |
| **전략 스튜디오** | 비주얼 빌더 — 코인 소스, 지표, 리스크 관리 |
| **AI 토론 아레나** | 여러 AI가 거래 토론 (강세 vs 약세 vs 분석가), 투표, 실행 |
| **AI 경쟁** | AI가 실시간 경쟁, 리더보드 순위 |
| **Telegram 에이전트** | 트레이딩 어시스턴트와 채팅 — 스트리밍, 도구 호출, 메모리 |
| **백테스트 랩** | 과거 시뮬레이션, 자산 곡선 및 성과 지표 |
| **대시보드** | 실시간 포지션, 손익, Chain of Thought AI 결정 로그 |

### 시장

암호화폐 · 미국 주식 · 외환 · 귀금속

### 거래소 (CEX)

| 거래소 | 상태 | 등록 (수수료 할인) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [등록](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [등록](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [등록](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [등록](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [등록](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [등록](https://www.gatenode.xyz/share/VQBGUAxY) |

### 거래소 (Perp-DEX)

| 거래소 | 상태 | 등록 (수수료 할인) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [등록](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [등록](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [등록](https://app.lighter.xyz/?referral=68151432) |

### AI 모델 (API 키 모드)

| AI 모델 | 상태 | API 키 받기 |
|:---------|:------:|:------------|
| <img src="../../../web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [API 키 받기](https://platform.deepseek.com) |
| <img src="../../../web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **Qwen** | ✅ | [API 키 받기](https://dashscope.console.aliyun.com) |
| <img src="../../../web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [API 키 받기](https://platform.openai.com) |
| <img src="../../../web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [API 키 받기](https://console.anthropic.com) |
| <img src="../../../web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [API 키 받기](https://aistudio.google.com) |
| <img src="../../../web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [API 키 받기](https://console.x.ai) |
| <img src="../../../web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [API 키 받기](https://platform.moonshot.cn) |

### AI 모델 (x402 모드 — API 키 불필요)

15+ 모델을 [Claw402](https://claw402.ai) 또는 [BlockRun](https://blockrun.ai)으로 이용 — USDC 지갑만 있으면 됩니다

---

## 설치

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (클라우드)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### 소스에서

```bash
# 필수 조건: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # 백엔드
cd web && npm install && npm run dev  # 프론트엔드 (새 터미널)
```

---

## 링크

| | |
|:--|:--|
| 웹사이트 | [nofxai.com](https://nofxai.com) |
| 대시보드 | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API 문서 | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **위험 경고**: AI 자동 거래에는 상당한 위험이 있습니다. 학습/연구 또는 소액 테스트만 권장합니다.

---

## License

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
