# NOFX - AI 트레이딩 시스템

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

**언어:** [English](../../../README.md) | [中文](../zh-CN/README.md) | [한국어](README.md)

---

## AI 기반 암호화폐 거래 플랫폼

**NOFX**는 여러 AI 모델을 실행하여 암호화폐 선물을 자동으로 거래할 수 있는 오픈소스 AI 거래 시스템입니다. 웹 인터페이스를 통해 전략을 구성하고, 실시간으로 성과를 모니터링하며, AI 에이전트들이 최적의 거래 방식을 찾도록 경쟁시킵니다.

### 핵심 기능

- **다중 AI 지원**: DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi 실행 - 언제든 모델 전환 가능
- **다중 거래소**: Binance, Bybit, OKX, Hyperliquid, Aster DEX, Lighter에서 통합 거래
- **전략 스튜디오**: 코인 소스, 지표, 리스크 제어를 설정하는 시각적 전략 빌더
- **AI 경쟁 모드**: 여러 AI 트레이더가 실시간으로 경쟁, 성과를 나란히 추적
- **웹 기반 설정**: JSON 편집 불필요 - 웹 인터페이스에서 모든 설정 완료
- **실시간 대시보드**: 실시간 포지션, 손익 추적, 사고의 연쇄가 포함된 AI 결정 로그

### [Amber.ac](https://amber.ac) 후원

> **위험 경고**: 이 시스템은 실험적입니다. AI 자동 거래에는 상당한 위험이 있습니다. 학습/연구 목적 또는 소액 테스트만 강력히 권장합니다!

## 개발자 커뮤니티

Telegram 개발자 커뮤니티 참여: **[NOFX 개발자 커뮤니티](https://t.me/nofx_dev_community)**

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

## 라이선스

**GNU Affero General Public License v3.0 (AGPL-3.0)**

---

## 연락처

- **GitHub Issues**: [Issue 제출](https://github.com/NoFxAiOS/nofx/issues)
- **개발자 커뮤니티**: [Telegram 그룹](https://t.me/nofx_dev_community)
