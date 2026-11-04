<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — 거래로 이어지는 지능. 직접 운영하는 개방형 트레이딩 시스템." width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">시작하기</a> ·
  <a href="#documentation">문서</a> ·
  <a href="#community">커뮤니티</a>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=flat&labelColor=192434&color=295BE8" alt="GitHub stars"></a>
  <a href="../../../LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-295BE8?style=flat&labelColor=192434" alt="License: AGPL-3.0"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/telegram-community-295BE8?style=flat&labelColor=192434&logo=telegram&logoColor=white" alt="Telegram community"></a>
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

NOFX는 자신의 컴퓨터에서 실행하는 오픈 소스 트레이딩 시스템입니다. **Frontier**에서 AI와 거래 아이디어를 검토하고, 차트에서 진입 방식을 비교한 뒤 직접 주문을 승인하세요. **Autopilot**으로는 전략을 지속적으로 실행할 수 있습니다. 두 기능은 설정한 모델과 거래소 계정을 함께 사용합니다.

[Vergex](https://vergex.trade)와 함께 개발하며, AI와 데이터는 [Claw402](https://claw402.ai)를 통해 이용합니다.

<a id="run-locally"></a>

## 로컬에서 실행하기

**한 줄 설치 — Linux / macOS**

Compose가 포함된 Docker를 설치하고 실행한 뒤 다음 명령을 입력하세요.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

데스크톱 브라우저에서 **http://127.0.0.1:3000**을 여세요.

설치 프로그램이 누락된 환경 설정 키를 생성하고, 배포된 Docker 이미지로 NOFX를 실행합니다. Go나 Node.js를 별도로 설치할 필요가 없습니다. [처음 사용하기](#your-first-session)로 이동하거나 [소스 빌드 및 다른 설치 방법](#source-builds-and-other-options)을 확인하세요.

## Frontier

차트와 대화를 통해, 주문하기 전에 내용을 확인할 수 있는 거래 계획을 만듭니다.

1. **시장을 선택하세요.** 거래량 상위 종목, 거래소의 종목 코드 검색, Binance AI500 목록을 이용할 수 있습니다. 거래소가 지원하는 경우 주식 연계 종목도 표시됩니다.
2. **진입 방식을 비교하세요.** 시장가, 지정가, 가중 분할 주문, TWAP 계획을 요청하세요. 진입가, 손절가, 목표가, 레버리지, 포지션 가치, 예상 증거금을 함께 볼 수 있습니다.
3. **검토하고 주문하세요.** 계획을 선택하면 차트가 해당 계획에 초점을 맞춥니다. 현재 계정과 주문 내용을 확인한 뒤 실행을 명시적으로 승인하세요.
4. **포지션을 관리하세요.** 차트 아래에서 실시간 포지션과 거래소 주문을 확인하고, 수동으로 진입하거나 포지션을 청산할 수 있습니다.

Frontier는 **Binance USDT 무기한 선물, Hyperliquid, OKX, Bitget**을 지원합니다. 거래 계정을 연결하기 전에도 공개 시장 데이터를 볼 수 있지만, 주문하려면 해당 거래소 계정이 필요합니다. 계획 생성만으로 주문이 제출되지는 않습니다.

| 진입 방식 | 동작 |
| :--- | :--- |
| **시장가** | 승인 후 체결 가능한 시장 가격으로 진입합니다. 표시된 진입가는 추정치입니다. |
| **지정가** | 선택한 가격으로 진입 주문을 제출합니다. |
| **분할 주문** | 여러 가격 구간에 정해진 비중으로 포지션을 나눕니다. |
| **TWAP** | 승인된 가격 범위 내에서 정해진 횟수의 주문을 시간 간격에 따라 제출합니다. |

지정가 주문과 예약된 분할 주문은 체결되지 않을 수 있습니다. [실행 및 복구 안내](../../features/trade-planning.md)를 참고하세요.

## Autopilot

전략을 지속적으로 실행하려면 모델, 거래소 계정, 전략을 지정해 트레이더를 설정하세요. Autopilot은 시장을 읽고 판단한 뒤 NOFX의 거래소 실행 모듈을 통해 주문합니다. Dashboard에서 포지션, 성과, 의사결정 기록을 확인할 수 있습니다.

**Frontier는 거래마다 승인이 필요합니다. Autopilot은 실행 중 자동으로 주문할 수 있습니다.** Autopilot의 시작과 중지는 별도의 작업입니다.

| 작업 공간 | 용도 |
| :--- | :--- |
| **Config** | 모델 연결, AI 지갑 충전, 거래 계정 관리. |
| **Frontier** | 시장 탐색, 진입 계획 비교, 검토한 주문 실행. |
| **Dashboard** | 실행 중인 트레이더, 포지션, 의사결정 기록 확인. |
| **Strategy** | 종목 선택, 지표, 포지션 크기, 거래 지침 설정. |
| **Leaderboard** | 공개된 트레이더 성과 비교. |

<a id="your-first-session"></a>

## 처음 사용하기

1. **로컬 계정을 만드세요.** 방금 실행한 NOFX에 로그인합니다.
2. **Config에서 AI를 설정하세요.** 모델을 선택합니다. Claw402를 사용한다면 AI 이용료 지갑을 생성하거나 가져오고, 개인 키를 보관한 뒤 해당 주소로 **Base 네트워크의 USDC**를 보내세요. 이 잔액은 모델 및 유료 데이터 요청에 사용됩니다.
3. **거래 계정을 추가하세요.** 사용할 거래소를 연결하고 계정 자산과 사용 가능 잔액을 확인합니다. 거래 자금은 AI 이용료 지갑과 별도로 관리됩니다.
4. **Frontier를 여세요.** 거래소, 계정, 종목을 선택합니다. 계획을 생성하고 차트에서 비교한 다음 준비가 되면 주문을 검토하세요.

지속적인 자동 거래를 하려면 Autopilot을 별도로 설정하고 시작하세요.

### 설치 프로그램으로 설치한 환경 업데이트

실행 중인 Autopilot을 중지하고 진행 중인 Frontier 거래를 확인한 뒤, 설치 디렉터리(기본값 `~/nofx`)에서 다음 명령을 실행하세요.

```bash
docker compose pull
docker compose up -d
```

업데이트할 때 `.env`와 `data/` 디렉터리를 보존하세요. 설정, 암호화에 필요한 자료, 저장된 애플리케이션 상태가 포함되어 있습니다.

재시작 후 거래소에서 포지션과 주문을 확인하고, 확인이 필요한 실행 기록을 대조한 뒤 다음 거래를 진행하세요. 프로세스를 중지해도 포지션이 청산되지는 않습니다.

<a id="source-builds-and-other-options"></a>

## 소스 빌드 및 다른 설치 방법

<details>
<summary>개발자: 소스에서 빌드하고 업데이트하기</summary>

NOFX를 개발하거나 배포 이미지에 반영되기 전의 변경 사항을 테스트할 때 사용하세요. Git과 Compose가 포함된 Docker를 설치하고 Docker를 실행합니다.

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

시작 스크립트는 필요한 경우 `.env`를 만들고 누락된 암호화 키를 생성합니다. 서브모듈 없이 복제했다면 빌드 전에 `git submodule update --init web/vendor/charting`을 실행하세요. TradingView 차트 리소스에는 해당 라이선스가 적용됩니다. [차트 통합 안내](../../features/trade-planning.md#harness-chart-and-plugins)를 참고하세요.

소스로 설치한 환경을 업데이트하려면 실행 중인 Autopilot을 중지하고 진행 중인 Frontier 거래를 확인한 뒤 다음을 실행합니다.

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows, Railway 및 원격 배포</summary>

**Windows**

Docker Desktop과 WSL2 같은 Bash 환경을 준비한 뒤 위의 한 줄 설치 명령을 실행하세요.

**Railway**

[![Railway에 배포](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**원격 배포**

설치 프로그램은 프로덕션 Compose 설정을 사용하며, 기본적으로 호스트의 모든 네트워크 인터페이스에 포트를 공개합니다. 소스의 Compose 설정은 프런트엔드와 API를 기본적으로 `127.0.0.1`에 바인딩합니다. 원격 인스턴스를 외부에 공개하기 전에 바인딩 주소와 인증 기능을 갖춘 HTTPS 프록시를 설정하세요.

</details>

<details>
<summary>Docker 없이 개발하기</summary>

[go.mod](../../../go.mod)에 지정된 Go 버전, 프런트엔드 Docker 빌드에서 사용하는 Node.js 24, TA-Lib을 사용합니다. 프런트엔드 시작 전에 차트 서브모듈을 초기화하고 [.env.example](../../../.env.example)을 참고해 환경을 설정하세요.

```bash
# 백엔드
go build -o nofx .
./nofx

# 프런트엔드: 다른 터미널에서 실행
cd web
npm ci
npm run dev
```

</details>

## 모델과 데이터

Frontier와 Autopilot은 NOFX에 설정된 모델을 함께 사용합니다. [Claw402](https://claw402.ai)는 여러 모델을 제공하며 Base 네트워크의 USDC로 사용량에 따라 정산합니다. Config에서 모델을 선택하고 작업 공간에서 전환할 수 있습니다.

가격과 캔들 데이터는 선택한 거래소에서 가져옵니다. AI500과 지원되는 보유자 분석은 Claw402 유료 경로를 이용합니다. 데이터 제공 여부는 시장과 제공업체에 따라 달라집니다. Hyperliquid 보유자 데이터를 Binance, OKX, Bitget 데이터로 대신 사용하지 않습니다. 활동 스트림에서 데이터 조회와 계획 생성 진행 상황을 확인할 수 있습니다.

AI 이용료, 유료 데이터 요금, 거래소 자금은 별개입니다. 거래가 실행되지 않아도 모델 요청에는 요금이 발생할 수 있습니다.

## 거래소

현재 Frontier에서 선택할 수 있는 거래소는 Binance, Hyperliquid, OKX, Bitget입니다. NOFX의 전체 실행 모듈에는 Bybit, Gate, KuCoin, Aster, Lighter도 포함됩니다. 이용 가능한 상품, 주문 유형, 계정 권한은 거래소마다 다릅니다.

아래는 개발을 지원하는 파트너 가입 링크입니다. 수수료 혜택은 각 거래소가 정합니다.

| 거래소 | 지원 | 파트너 링크 |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [가입](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [가입](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [가입](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [가입](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [가입](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [가입](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [가입](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [가입](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [가입](https://app.lighter.xyz/?referral=68151432)                              |

## 주문 실행 방식

계획과 전략의 결정은 기존 NOFX 거래소 어댑터를 통해 실행됩니다. 모델 프롬프트에는 지갑 개인 키나 거래소 인증 정보가 포함되지 않습니다.

- **승인 전 검토.** Frontier는 계정, 주문 규모, 진입 및 청산 정보를 표시합니다. Autopilot은 설정한 전략과 실행 제한을 따릅니다.
- **계정에 맞춘 주문 규모.** Frontier는 선택한 레버리지와 현재 사용 가능한 증거금을 기준으로 계산하며 실행 비용을 남겨 둡니다. 수량 정밀도, 호가 단위, 거래소 제한도 적용됩니다.
- **거래소에 제출하는 보호 주문.** 체결을 확인하면 지원되는 손절 및 익절 주문을 제출합니다. 진입과 보호 주문은 하나의 원자적 작업이 아니므로 보호 주문에 실패하면 거래소에서 확인하세요.
- **주문 상태 대조.** 제출 결과가 불확실하면 무조건 재전송하지 않고 거래소에서 확인합니다. 해결되지 않은 실행 상태가 있으면 추가 진입이 일시 중지될 수 있습니다.
- **로컬 설정.** 거래소 인증 정보는 NOFX 서버에 암호화되어 저장됩니다. 설치 환경에는 서명에 필요한 자료도 있으므로 파일과 백업을 보호하세요.

## 아키텍처

```text
데스크톱 브라우저
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier: 계획, 검토, 승인
      → Autopilot: 전략 반복 실행
    → 공통 실행 모듈 → 거래소 어댑터 → 사용자 거래 계정

모델과 유료 데이터: NOFX 공급자 클라이언트 · Claw402
저장: 로컬 설정, 계획, 실행 기록
```

<details>
<summary>이전 인터페이스 데모</summary>

이 영상은 이전 버전의 인터페이스를 보여 줍니다. 현재는 탐색 메뉴와 디자인이 변경되었습니다.

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## 브랜드 자료

- [로고 심벌 — SVG](../../../web/public/icons/nofx-flow.svg)
- [GitHub 아바타 — PNG, 512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [GitHub 아바타 — SVG 원본](../../../web/public/brand/nofx-avatar.svg)
- [README 배너 — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## 문서

| 가이드 | 내용 |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | 계획, 실거래 실행, 데이터, 복구 |
| [시작하기](../../getting-started/README.md) | 배포 및 거래소 API 안내 |
| [아키텍처](../../architecture/README.md) | 시스템 설계와 모듈 목록 |
| [전략 모듈](../../architecture/STRATEGY_MODULE.md) | 종목 선택, AI 프롬프트, 실행 |
| [자주 묻는 질문](../../guides/faq.en.md) | 일반적인 질문과 답변 |
| [문제 해결](../../guides/TROUBLESHOOTING.md) | 일반적인 문제 진단 |

<a id="community"></a>

## 커뮤니티

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [라이브 대시보드](https://vergex.trade/explore)

## 기여

코드, 문서, 번역, 버그 리포트 모두 환영합니다 — [기여 가이드](../../../CONTRIBUTING.md), [행동 강령](../../../CODE_OF_CONDUCT.md), [보안 정책](../../../SECURITY.md)을 참고하세요.

NOFX는 의미 있는 기여를 기록하며, 생태계가 성장함에 따라 기여자에게 보상할 계획입니다. 우선순위 이슈에는 더 높은 가중치가 부여됩니다.

| 기여 유형      | 가중치 |
| :---------------- | :----: |
| 고정 이슈 PR  | ★★★★★★ |
| 코드(머지된 PR) | ★★★★★  |
| 버그 수정         |  ★★★★  |
| 기능 아이디어     |  ★★★   |
| 버그 리포트       |   ★★   |
| 문서     |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

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

<br/>

NOFX가 도움이 되었다면, 스타 하나가 다른 트레이더들이 이 프로젝트를 발견하는 데 큰 힘이 됩니다.

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## 라이선스

[AGPL-3.0](../../../LICENSE)

<sub>자동매매에는 상당한 위험이 따릅니다. AI 기반 전략은 실험적이며 손실이 발생할 수 있습니다. 포지션 규모를 적절히 관리하고, 각 거래소의 구조를 이해하며, 잃어도 감당할 수 있는 자금으로만 거래하세요. 전체 [면책 조항](../../../DISCLAIMER.md)을 확인하세요.</sub>
