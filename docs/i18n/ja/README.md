# NOFX - AI トレーディングシステム

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

**言語:** [English](../../../README.md) | [中文](../zh-CN/README.md) | [日本語](README.md)

---

## AI 駆動の暗号通貨取引プラットフォーム

**NOFX** は、複数の AI モデルを使用して暗号通貨先物を自動取引できるオープンソースの AI 取引システムです。Web インターフェースで戦略を設定し、リアルタイムでパフォーマンスを監視し、AI エージェントを競わせて最適な取引アプローチを見つけます。

### コア機能

- **マルチ AI サポート**: DeepSeek、Qwen、GPT、Claude、Gemini、Grok、Kimi を実行 - いつでもモデルを切り替え可能
- **マルチ取引所**: Binance、Bybit、OKX、Hyperliquid、Aster DEX、Lighter で統一取引
- **ストラテジースタジオ**: コインソース、インジケーター、リスク管理を設定するビジュアル戦略ビルダー
- **AI 競争モード**: 複数の AI トレーダーがリアルタイムで競争、パフォーマンスを並べて追跡
- **Web ベース設定**: JSON 編集不要 - Web インターフェースですべて設定
- **リアルタイムダッシュボード**: ライブポジション、損益追跡、思考連鎖付き AI 決定ログ

### [Amber.ac](https://amber.ac) 支援

> **リスク警告**: このシステムは実験的です。AI 自動取引には重大なリスクがあります。学習/研究目的または少額でのテストのみを強くお勧めします！

## 開発者コミュニティ

Telegram 開発者コミュニティに参加: **[NOFX 開発者コミュニティ](https://t.me/nofx_dev_community)**

---

## クイックスタート

### オプション 1: Docker デプロイ（推奨）

```bash
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
chmod +x ./start.sh
./start.sh start --build
```

Web インターフェースにアクセス: **http://localhost:3000**

### オプション 2: 手動インストール

```bash
# 前提条件: Go 1.21+, Node.js 18+, TA-Lib

# TA-Lib インストール (macOS)
brew install ta-lib

# クローンとセットアップ
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
go mod download
cd web && npm install && cd ..

# バックエンド起動
go build -o nofx && ./nofx

# フロントエンド起動（新しいターミナル）
cd web && npm run dev
```

---

## 初期設定

1. **AI モデル設定** - AI API キーを追加
2. **取引所設定** - 取引所 API 認証情報を設定
3. **戦略作成** - ストラテジースタジオで取引戦略を設定
4. **トレーダー作成** - AI モデル + 取引所 + 戦略を組み合わせ
5. **取引開始** - 設定したトレーダーを起動

---

## リスク警告

1. 暗号通貨市場は非常に変動が激しい - AI の決定は利益を保証しない
2. 先物取引はレバレッジを使用 - 損失は元本を超える可能性がある
3. 極端な市場状況では清算リスクがある

---

## ライセンス

**GNU Affero General Public License v3.0 (AGPL-3.0)**

---

## コンタクト

- **GitHub Issues**: [Issue を提出](https://github.com/NoFxAiOS/nofx/issues)
- **開発者コミュニティ**: [Telegram グループ](https://t.me/nofx_dev_community)
