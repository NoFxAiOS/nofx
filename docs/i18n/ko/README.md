<p align="center"><strong><a href="https://vergex.trade">vergex.trade</a>가 지원합니다</strong></p>

<h1 align="center">NOFX</h1>

<p align="center">
  <strong>글로벌 시장을 위한 AI 트레이딩 터미널.</strong><br/>
  <strong>미국 주식, 원자재, 외환, 암호화폐 리서치, 전략 생성, 실행, 모니터링.</strong>
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

NOFX는 시장 리서치, 전략 개발, 거래 실행, 포트폴리오 모니터링을 하나의 워크스페이스에서 처리하는 오픈소스 AI 트레이딩 터미널입니다.

제품은 미국 주식, 원자재 계약, FX 페어, 디지털 자산 등 글로벌 유동성 시장을 중심으로 설계되었습니다. AI 레이어는 거래 의도를 워치리스트, 신호, 전략 로직, 리스크 제어, 실행 워크플로로 변환합니다.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

**http://127.0.0.1:3000** 을 엽니다.

---

## 거래소 등록

아래 링크를 통해 암호화폐와 지원되는 미국 주식, FX, 원자재 파생상품 시장용 거래 계정을 개설할 수 있습니다. 이 링크는 NOFX 파트너 프로그램을 통해 제공되며 수수료 할인 또는 추천 혜택이 포함될 수 있습니다.

| 거래소 | 상태 | 수수료 할인 등록 |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [등록](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [등록](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [등록](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [등록](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [등록](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [등록](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [등록](https://www.gatenode.xyz/share/VQBGUAxY) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster** | ✅ | [등록](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [등록](https://app.lighter.xyz/?referral=68151432) |

---

## 빠른 데모

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

---

## 시장

**미국 주식 · 원자재 · 외환 · 암호화폐**

NOFX는 단일 거래소 화면이 아니라 멀티에셋 리서치, 전략 구축, 실행, 모니터링 워크플로를 중심으로 구성됩니다.

---

## AI 모델 액세스

NOFX는 AI 추론을 [Claw402](https://claw402.ai)를 통해 자동 라우팅합니다. 사용자는 모델 제공업체를 설정하거나 API 키를 관리하거나 별도 AI 계정을 유지할 필요가 없습니다. 터미널은 Claw402의 사용량 기반 인프라를 통해 지원 모델에 온디맨드로 접근하며 공식 할인 채널로 트래픽을 라우팅합니다.

| 제공업체 | 액세스 |
| :--- | :--- |
| **Claw402** | [공식 할인으로 사용량 기반 AI 모델 이용](https://claw402.ai) |

---

## 기능

| 기능 | 설명 |
| :--- | :--- |
| **AI 트레이딩 터미널** | 미국 주식, 원자재, 외환, 암호화폐 워크플로를 위한 통합 워크스페이스 |
| **AI 모델 액세스** | Claw402를 통해 지원 모델 제공업체에 자동 연결 |
| **거래소 연결** | Binance, Bybit, OKX, Hyperliquid, Bitget, KuCoin, Gate, Aster, Lighter |
| **Strategy Studio** | 시장 유니버스, 지표, 리스크 제어, 전략 로직 |
| **모델 경쟁** | AI 트레이더의 실시간 성과와 리더보드 비교 |
| **Telegram Agent** | 채팅으로 트레이딩 어시스턴트 제어 및 모니터링 |
| **포트폴리오 대시보드** | 포지션, 손익, 실행 기록, 모델 의사결정 로그 |

---

## 스크린샷

<details>
<summary><b>설정 페이지</b></summary>

|                         설정                         |                         트레이더 목록                         |
| :----------------------------------------------------: | :----------------------------------------------------------: |
| <img src="../../../screenshots/config-ai-exchanges.png" width="400"/> | <img src="../../../screenshots/config-traders-list.png" width="400"/> |

</details>

<details>
<summary><b>대시보드</b></summary>

|                        개요                         |                          마켓 차트                           |
| :-----------------------------------------------------: | :-------------------------------------------------------------: |
| <img src="../../../screenshots/dashboard-page.png" width="400"/> | <img src="../../../screenshots/dashboard-market-chart.png" width="400"/> |

|                          거래 통계                           |                          포지션 기록                           |
| :--------------------------------------------------------------: | :-----------------------------------------------------------------: |
| <img src="../../../screenshots/dashboard-trading-stats.png" width="400"/> | <img src="../../../screenshots/dashboard-position-history.png" width="400"/> |

|                          포지션                           |                    트레이더 상세                     |
| :----------------------------------------------------------: | :---------------------------------------------------: |
| <img src="../../../screenshots/dashboard-positions.png" width="400"/> | <img src="../../../screenshots/details-page.png" width="400"/> |

</details>

<details>
<summary><b>Strategy Studio</b></summary>

|                     전략 에디터                      |                      지표 설정                       |
| :------------------------------------------------------: | :----------------------------------------------------------: |
| <img src="../../../screenshots/strategy-studio.png" width="400"/> | <img src="../../../screenshots/strategy-indicators.png" width="400"/> |

</details>

<details>
<summary><b>경쟁</b></summary>

|                     경쟁 모드                      |
| :-------------------------------------------------------: |
| <img src="../../../screenshots/competition-page.png" width="400"/> |

</details>

---

## 설치

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway(클라우드)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### Windows

[Docker Desktop](https://www.docker.com/products/docker-desktop/) 설치 후:

```powershell
curl -o docker-compose.prod.yml https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### 소스에서 빌드

```bash
# Prerequisites: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib
# Ubuntu: sudo apt-get install libta-lib0-dev

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx
cd web && npm install && npm run dev
```

### 업데이트

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

---

## 설정

**초보자 모드**: 가이드 온보딩이 모델 액세스, 거래소 연결, 전략 설정, 첫 배포까지 안내합니다.

**고급 모드**:

1. AI 모델 액세스 설정
2. 거래소 인증 정보 연결
3. 전략 생성 또는 가져오기
4. AI 트레이더 프로필 생성
5. 대시보드에서 실행, 모니터링, 개선

모든 설정은 Web UI **http://127.0.0.1:3000** 에서 가능합니다.

---

## 서버에 배포

**HTTP 배포:**

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
# http://YOUR_IP:3000 으로 접근
```

**Cloudflare를 통한 HTTPS:**

1. [Cloudflare](https://dash.cloudflare.com)(무료 플랜)에 도메인 추가
2. A 레코드를 서버 IP로 지정(Proxied)
3. SSL/TLS를 Flexible로 설정
4. `.env`에 `TRANSPORT_ENCRYPTION=true` 설정

---

## 아키텍처

```
                              NOFX
    ┌─────────────────────────────────────────────────┐
    │                 Trading Terminal                 │
    │        React + TypeScript + TradingView          │
    │      US Stocks · Commodities · Forex · Crypto    │
    ├─────────────────────────────────────────────────┤
    │                  API Server (Go)                  │
    ├──────────────┬──────────────┬───────────────────┤
    │   Strategy    │   Telegram   │   Trader Runtime  │
    │    Engine     │    Agent     │   Risk Controls   │
    ├──────────────┴──────────────┴───────────────────┤
    │                 AI Model Layer                    │
    │    Unified provider access through Claw402        │
    │    Model routing · payment · execution support    │
    ├─────────────────────────────────────────────────┤
    │              Exchange Connectivity                │
    │ Binance · Bybit · OKX · Hyperliquid · Bitget     │
    │ KuCoin · Gate · Aster · Lighter                  │
    └─────────────────────────────────────────────────┘
```

---

## 문서

| | |
| :--- | :--- |
| [아키텍처](../../architecture/README.md) | 시스템 설계와 모듈 색인 |
| [전략 모듈](../../architecture/STRATEGY_MODULE.md) | 종목 선택, AI 프롬프트, 실행 |
| [FAQ](../../faq/README.md) | 자주 묻는 질문 |
| [시작하기](../../getting-started/README.md) | 배포 가이드 |

---

## 기여

[기여 가이드](../../../CONTRIBUTING.md), [행동 강령](../../../CODE_OF_CONDUCT.md), [보안 정책](../../../SECURITY.md)을 확인하세요.

### 기여자 프로그램

NOFX는 의미 있는 기여를 기록하며 생태계 성장에 따라 기여자에게 보상할 계획입니다. 우선순위 이슈는 더 높은 보상 가중치를 가집니다.

| Contribution | Weight |
| :--- | :---: |
| Pinned Issue PRs | ★★★★★★ |
| Code (Merged PRs) | ★★★★★ |
| Bug Fixes | ★★★★ |
| Feature Ideas | ★★★ |
| Bug Reports | ★★ |
| Documentation | ★★ |

---

## 링크

| | |
| :--- | :--- |
| Website | [vergex.trade](https://vergex.trade) |
| Dashboard | [vergex.trade/explore](https://vergex.trade/explore) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@vergex_ai](https://x.com/vergex_ai) |

> **위험 고지**: 자동매매에는 상당한 위험이 따릅니다. 적절한 포지션 규모를 사용하고 각 거래소 구조를 이해하며 감당 가능한 자금만 거래하세요.

---

## 스폰서

<a href="https://github.com/pjl914335852-ux"><img src="https://github.com/pjl914335852-ux.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cat9999aaa"><img src="https://github.com/cat9999aaa.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/1733055465"><img src="https://github.com/1733055465.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/kolal2020"><img src="https://github.com/kolal2020.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/CyberFFarm"><img src="https://github.com/CyberFFarm.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/vip3001003"><img src="https://github.com/vip3001003.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/mrtluh"><img src="https://github.com/mrtluh.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cpcp1117-source"><img src="https://github.com/cpcp1117-source.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/match-007"><img src="https://github.com/match-007.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/leiwuhen1715"><img src="https://github.com/leiwuhen1715.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/SHAOXIA1991"><img src="https://github.com/SHAOXIA1991.png" width="50" height="50" style="border-radius:50%"/></a>

[스폰서 되기](https://github.com/sponsors/NoFxAiOS)

## License

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
