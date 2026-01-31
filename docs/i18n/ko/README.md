<h1 align="center">NOFX — 오픈소스 AI 트레이딩 OS</h1>

<p align="center">
  <strong>AI 기반 금융 거래를 위한 인프라 레이어</strong>
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
  <a href="https://www.typescriptlang.org/"><img src="https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript" alt="TypeScript"></a>
</p>

**언어:** [English](../../../README.md) | [中文](../zh-CN/README.md) | [한국어](README.md)

---

### 핵심 기능

- **다중 AI 지원**: DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi 실행 - 언제든 모델 전환 가능
- **다중 거래소**: Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster DEX, Lighter에서 통합 거래
- **전략 스튜디오**: 코인 소스, 지표, 리스크 제어를 설정하는 시각적 전략 빌더
- **AI 경쟁 모드**: 여러 AI 트레이더가 실시간으로 경쟁, 성과를 나란히 추적
- **웹 기반 설정**: JSON 편집 불필요 - 웹 인터페이스에서 모든 설정 완료
- **실시간 대시보드**: 실시간 포지션, 손익 추적, 사고의 연쇄가 포함된 AI 결정 로그

### 공식 링크

- **공식 웹사이트**: [https://nofxai.com](https://nofxai.com)
- **데이터 대시보드**: [https://nofxos.ai/dashboard](https://nofxos.ai/dashboard)
- **API 문서**: [https://nofxos.ai/api-docs](https://nofxos.ai/api-docs)

> **위험 경고**: 이 시스템은 실험적입니다. AI 자동 거래에는 상당한 위험이 있습니다. 학습/연구 목적 또는 소액 테스트만 강력히 권장합니다!

## 개발자 커뮤니티

Telegram 개발자 커뮤니티 참여: **[NOFX 개발자 커뮤니티](https://t.me/nofx_dev_community)**

---

## 시작하기 전에

NOFX를 사용하려면 다음이 필요합니다:

1. **거래소 계정** - 지원되는 거래소에 등록하고 거래 권한이 있는 API 자격 증명 생성
2. **AI 모델 API 키** - 지원되는 제공업체에서 획득 (비용 효율성을 위해 DeepSeek 권장)

---

## 지원 거래소

### CEX (중앙화 거래소)

| 거래소 | 상태 | 등록 (수수료 할인) |
|----------|--------|-------------------------|
| **Binance** | ✅ 지원 | [등록](https://www.binance.com/join?ref=NOFXENG) |
| **Bybit** | ✅ 지원 | [등록](https://partner.bybit.com/b/83856) |
| **OKX** | ✅ 지원 | [등록](https://www.okx.com/join/1865360) |
| **Bitget** | ✅ 지원 | [등록](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| **KuCoin** | ✅ 지원 | [등록](https://www.kucoin.com/r/broker/CXEV7XKK) |
| **Gate** | ✅ 지원 | [등록](https://www.gatenode.xyz/share/VQBGUAxY) |

### Perp-DEX (탈중앙화 영구 선물 거래소)

| 거래소 | 상태 | 등록 (수수료 할인) |
|----------|--------|-------------------------|
| **Hyperliquid** | ✅ 지원 | [등록](https://app.hyperliquid.xyz/join/AITRADING) |
| **Aster DEX** | ✅ 지원 | [등록](https://www.asterdex.com/en/referral/fdfc0e) |
| **Lighter** | ✅ 지원 | [등록](https://app.lighter.xyz/?referral=68151432) |

---

## 지원 AI 모델

| AI 모델 | 상태 | API 키 받기 |
|----------|--------|-------------|
| **DeepSeek** | ✅ 지원 | [API 키 받기](https://platform.deepseek.com) |
| **Qwen** | ✅ 지원 | [API 키 받기](https://dashscope.console.aliyun.com) |
| **OpenAI (GPT)** | ✅ 지원 | [API 키 받기](https://platform.openai.com) |
| **Claude** | ✅ 지원 | [API 키 받기](https://console.anthropic.com) |
| **Gemini** | ✅ 지원 | [API 키 받기](https://aistudio.google.com) |
| **Grok** | ✅ 지원 | [API 키 받기](https://console.x.ai) |
| **Kimi** | ✅ 지원 | [API 키 받기](https://platform.moonshot.cn) |

---

## 빠른 시작

### 옵션 1: Docker 배포 (권장)

```bash
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
chmod +x ./start.sh
./start.sh start --build
```

웹 인터페이스 접속: **http://localhost:3000**

### 최신 버전 유지

> **💡 업데이트가 빈번합니다.** 최신 기능과 수정 사항을 받으려면 매일 이 명령을 실행하세요:

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

이 명령은 최신 공식 이미지를 가져오고 서비스를 자동으로 다시 시작합니다.

### 옵션 2: 수동 설치

```bash
# 필수 조건: Go 1.21+, Node.js 18+, TA-Lib

# TA-Lib 설치 (macOS)
brew install ta-lib

# 클론 및 설정
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
go mod download
cd web && npm install && cd ..

# 백엔드 시작
go build -o nofx && ./nofx

# 프론트엔드 시작 (새 터미널)
cd web && npm run dev
```

---

## 초기 설정

1. **AI 모델 설정** - AI API 키 추가
2. **거래소 설정** - 거래소 API 자격 증명 설정
3. **전략 생성** - 전략 스튜디오에서 거래 전략 구성
4. **트레이더 생성** - AI 모델 + 거래소 + 전략 조합
5. **거래 시작** - 설정된 트레이더 시작

---

## 위험 경고

1. 암호화폐 시장은 매우 변동성이 높음 - AI 결정이 수익을 보장하지 않음
2. 선물 거래는 레버리지 사용 - 손실이 원금을 초과할 수 있음
3. 극단적인 시장 상황에서 청산 위험 있음

---

## 서버 배포

### 빠른 배포 (IP를 통한 HTTP)

기본적으로 전송 암호화가 **비활성화**되어 HTTPS 없이 IP 주소를 통해 NOFX에 액세스할 수 있습니다:

```bash
# 서버에 배포
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

`http://YOUR_SERVER_IP:3000`을 통해 액세스 - 즉시 작동합니다.

### 향상된 보안 (HTTPS)

보안을 강화하려면 `.env`에서 전송 암호화를 활성화하세요:

```bash
TRANSPORT_ENCRYPTION=true
```

활성화되면 브라우저는 Web Crypto API를 사용하여 전송 전에 API 키를 암호화합니다. 이를 위해 필요한 것:
- `https://` - SSL이 있는 모든 도메인
- `http://localhost` - 로컬 개발

### Cloudflare를 사용한 빠른 HTTPS 설정

1. **Cloudflare에 도메인 추가** (무료 플랜 가능)
   - [dash.cloudflare.com](https://dash.cloudflare.com) 방문
   - 도메인 추가 및 네임서버 업데이트

2. **DNS 레코드 생성**
   - 유형: `A`
   - 이름: `nofx` (또는 서브도메인)
   - 콘텐츠: 서버 IP
   - 프록시 상태: **Proxied** (주황색 구름)

3. **SSL/TLS 구성**
   - SSL/TLS 설정으로 이동
   - 암호화 모드를 **Flexible**로 설정

   ```
   User ──[HTTPS]──→ Cloudflare ──[HTTP]──→ Your Server:3000
   ```

4. **전송 암호화 활성화**
   ```bash
   # .env 편집 및 설정
   TRANSPORT_ENCRYPTION=true
   ```

5. **완료!** `https://nofx.yourdomain.com`을 통해 액세스

---

## 초기 설정 (웹 인터페이스)

시스템을 시작한 후 웹 인터페이스를 통해 구성합니다:

1. **AI 모델 구성** - AI API 키 추가 (DeepSeek, OpenAI 등)
2. **거래소 구성** - 거래소 API 자격 증명 설정
3. **전략 생성** - 전략 스튜디오에서 거래 전략 구성
4. **트레이더 생성** - AI 모델 + 거래소 + 전략 결합
5. **거래 시작** - 구성된 트레이더 시작

모든 구성은 웹 인터페이스를 통해 완료 - JSON 파일 편집 불필요.

---

## 웹 인터페이스 기능

### 경쟁 페이지
- 실시간 ROI 리더보드
- 다중 AI 성능 비교 차트
- 실시간 손익 추적 및 순위

### 대시보드
- TradingView 스타일 캔들스틱 차트
- 실시간 포지션 관리
- Chain of Thought 추론이 포함된 AI 결정 로그
- 자본 곡선 추적

### 전략 스튜디오
- 코인 소스 구성 (정적 목록, AI500 풀, OI Top)
- 기술 지표 (EMA, MACD, RSI, ATR, 거래량, OI, 펀딩 비율)
- 리스크 제어 설정 (레버리지, 포지션 한도, 마진 사용)
- 실시간 프롬프트 미리보기를 포함한 AI 테스트

---

## 일반적인 문제

### TA-Lib을 찾을 수 없음
```bash
# macOS
brew install ta-lib

# Ubuntu
sudo apt-get install libta-lib0-dev
```

### AI API 타임아웃
- API 키가 올바른지 확인
- 네트워크 연결 확인
- 시스템 타임아웃은 120초

### 프론트엔드가 백엔드에 연결할 수 없음
- 백엔드가 http://localhost:8080에서 실행 중인지 확인
- 포트가 점유되어 있지 않은지 확인

---

## 라이선스

이 프로젝트는 **GNU Affero General Public License v3.0 (AGPL-3.0)** 라이선스에 따라 제공됩니다 - [LICENSE](LICENSE) 파일을 참조하세요.

---

## 기여

기여를 환영합니다! 다음을 참조하세요:
- **[기여 가이드](CONTRIBUTING.md)** - 개발 워크플로 및 PR 프로세스
- **[행동 강령](CODE_OF_CONDUCT.md)** - 커뮤니티 가이드라인
- **[보안 정책](SECURITY.md)** - 취약점 보고

---

## 기여자 에어드롭 프로그램

모든 기여는 GitHub에서 추적됩니다. NOFX가 수익을 창출하면 기여자는 기여도에 따라 에어드롭을 받게 됩니다.

**[고정된 Issue](https://github.com/NoFxAiOS/nofx/issues)를 해결하는 PR은 최고 보상을 받습니다!**

| 기여 유형 | 가중치 |
|------------------|:------:|
| **고정된 Issue PR** | ⭐⭐⭐⭐⭐⭐ |
| **코드 커밋** (병합된 PR) | ⭐⭐⭐⭐⭐ |
| **버그 수정** | ⭐⭐⭐⭐ |
| **기능 제안** | ⭐⭐⭐ |
| **버그 보고** | ⭐⭐ |
| **문서** | ⭐⭐ |

---

## 위험 경고

1. 암호화폐 시장은 매우 변동성이 높음 - AI 결정이 수익을 보장하지 않음
2. 선물 거래는 레버리지 사용 - 손실이 원금을 초과할 수 있음
3. 극단적인 시장 상황에서 청산 위험 있음




## 연락처

- **GitHub Issues**: [Issue 제출](https://github.com/NoFxAiOS/nofx/issues)
- **개발자 커뮤니티**: [Telegram 그룹](https://t.me/nofx_dev_community)

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
