<h1 align="center">NOFX — オープンソース AI トレーディング OS</h1>

<p align="center">
  <strong>AI 駆動金融取引のインフラストラクチャレイヤー</strong>
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

**言語:** [English](../../../README.md) | [中文](../zh-CN/README.md) | [日本語](README.md)

---

### コア機能

- **マルチ AI サポート**: DeepSeek、Qwen、GPT、Claude、Gemini、Grok、Kimi を実行 - いつでもモデルを切り替え可能
- **マルチ取引所**: Binance、Bybit、OKX、Bitget、KuCoin、Gate、Hyperliquid、Aster DEX、Lighter で統一取引
- **ストラテジースタジオ**: コインソース、インジケーター、リスク管理を設定するビジュアル戦略ビルダー
- **AI 競争モード**: 複数の AI トレーダーがリアルタイムで競争、パフォーマンスを並べて追跡
- **Web ベース設定**: JSON 編集不要 - Web インターフェースですべて設定
- **リアルタイムダッシュボード**: ライブポジション、損益追跡、思考連鎖付き AI 決定ログ

### 公式リンク

- **公式サイト**: [https://nofxai.com](https://nofxai.com)
- **データダッシュボード**: [https://nofxos.ai/dashboard](https://nofxos.ai/dashboard)
- **API ドキュメント**: [https://nofxos.ai/api-docs](https://nofxos.ai/api-docs)

> **リスク警告**: このシステムは実験的です。AI 自動取引には重大なリスクがあります。学習/研究目的または少額でのテストのみを強くお勧めします！

## 開発者コミュニティ

Telegram 開発者コミュニティに参加: **[NOFX 開発者コミュニティ](https://t.me/nofx_dev_community)**

---

## 始める前に

NOFXを使用するには以下が必要です:

1. **取引所アカウント** - サポートされている取引所に登録し、取引権限付きのAPI認証情報を作成
2. **AI モデル API キー** - サポートされているプロバイダーから取得（コスト効率の良いDeepSeekを推奨）

---

## サポート取引所

### CEX (中央集権型取引所)

| 取引所 | ステータス | 登録 (手数料割引) |
|----------|--------|-------------------------|
| **Binance** | ✅ サポート | [登録](https://www.binance.com/join?ref=NOFXENG) |
| **Bybit** | ✅ サポート | [登録](https://partner.bybit.com/b/83856) |
| **OKX** | ✅ サポート | [登録](https://www.okx.com/join/1865360) |
| **Bitget** | ✅ サポート | [登録](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| **KuCoin** | ✅ サポート | [登録](https://www.kucoin.com/r/broker/CXEV7XKK) |
| **Gate** | ✅ サポート | [登録](https://www.gatenode.xyz/share/VQBGUAxY) |

### Perp-DEX (分散型永久先物取引所)

| 取引所 | ステータス | 登録 (手数料割引) |
|----------|--------|-------------------------|
| **Hyperliquid** | ✅ サポート | [登録](https://app.hyperliquid.xyz/join/AITRADING) |
| **Aster DEX** | ✅ サポート | [登録](https://www.asterdex.com/en/referral/fdfc0e) |
| **Lighter** | ✅ サポート | [登録](https://app.lighter.xyz/?referral=68151432) |

---

## サポート AI モデル

| AI モデル | ステータス | API キー取得 |
|----------|--------|-------------|
| **DeepSeek** | ✅ サポート | [API キー取得](https://platform.deepseek.com) |
| **Qwen** | ✅ サポート | [API キー取得](https://dashscope.console.aliyun.com) |
| **OpenAI (GPT)** | ✅ サポート | [API キー取得](https://platform.openai.com) |
| **Claude** | ✅ サポート | [API キー取得](https://console.anthropic.com) |
| **Gemini** | ✅ サポート | [API キー取得](https://aistudio.google.com) |
| **Grok** | ✅ サポート | [API キー取得](https://console.x.ai) |
| **Kimi** | ✅ サポート | [API キー取得](https://platform.moonshot.cn) |

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

### 最新版への更新

> **💡 更新は頻繁です。** 最新の機能と修正を取得するために、毎日このコマンドを実行してください：

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

このコマンドは最新の公式イメージを取得し、サービスを自動的に再起動します。

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

## サーバー展開

### クイックデプロイ (HTTP経由のIP)

デフォルトでは、トランスポート暗号化は**無効**になっており、HTTPSなしでIPアドレス経由でNOFXにアクセスできます:

```bash
# サーバーにデプロイ
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

`http://YOUR_SERVER_IP:3000` 経由でアクセス - すぐに動作します。

### セキュリティ強化 (HTTPS)

セキュリティを強化するには、`.env`でトランスポート暗号化を有効にします:

```bash
TRANSPORT_ENCRYPTION=true
```

有効にすると、ブラウザはWeb Crypto APIを使用して転送前にAPIキーを暗号化します。これには以下が必要です:
- `https://` - SSLを備えた任意のドメイン
- `http://localhost` - ローカル開発

### Cloudflareを使用した簡単なHTTPSセットアップ

1. **ドメインをCloudflareに追加** (無料プランでOK)
   - [dash.cloudflare.com](https://dash.cloudflare.com) にアクセス
   - ドメインを追加してネームサーバーを更新

2. **DNSレコードを作成**
   - タイプ: `A`
   - 名前: `nofx` (またはサブドメイン)
   - コンテンツ: サーバーのIP
   - プロキシ状態: **Proxied** (オレンジ色の雲)

3. **SSL/TLSを設定**
   - SSL/TLS設定に移動
   - 暗号化モードを **Flexible** に設定

   ```
   User ──[HTTPS]──→ Cloudflare ──[HTTP]──→ Your Server:3000
   ```

4. **トランスポート暗号化を有効化**
   ```bash
   # .envを編集して設定
   TRANSPORT_ENCRYPTION=true
   ```

5. **完了!** `https://nofx.yourdomain.com` 経由でアクセス

---

## 初期設定 (Webインターフェース)

システムを起動した後、Webインターフェースを通じて設定します:

1. **AIモデルの設定** - AI APIキーを追加 (DeepSeek、OpenAI など)
2. **取引所の設定** - 取引所API認証情報を設定
3. **戦略の作成** - ストラテジースタジオで取引戦略を設定
4. **トレーダーの作成** - AIモデル + 取引所 + 戦略を組み合わせ
5. **取引開始** - 設定したトレーダーを起動

すべての設定はWebインターフェースで完了 - JSONファイルの編集は不要です。

---

## Webインターフェース機能

### 競争ページ
- リアルタイムROIリーダーボード
- マルチAIパフォーマンス比較チャート
- ライブ損益追跡とランキング

### ダッシュボード
- TradingViewスタイルのローソク足チャート
- リアルタイムポジション管理
- Chain of Thought推論付きAI決定ログ
- エクイティカーブ追跡

### ストラテジースタジオ
- コインソース設定 (静的リスト、AI500プール、OI Top)
- テクニカル指標 (EMA、MACD、RSI、ATR、出来高、OI、資金調達率)
- リスク管理設定 (レバレッジ、ポジション制限、証拠金使用率)
- リアルタイムプロンプトプレビュー付きAIテスト

---

## よくある問題

### TA-Libが見つからない
```bash
# macOS
brew install ta-lib

# Ubuntu
sudo apt-get install libta-lib0-dev
```

### AI APIタイムアウト
- APIキーが正しいか確認
- ネットワーク接続を確認
- システムタイムアウトは120秒

### フロントエンドがバックエンドに接続できない
- バックエンドが http://localhost:8080 で実行されているか確認
- ポートが占有されていないか確認

---

## ライセンス

このプロジェクトは **GNU Affero General Public License v3.0 (AGPL-3.0)** の下でライセンスされています - [LICENSE](LICENSE) ファイルを参照してください。

---

## 貢献

貢献を歓迎します！以下を参照してください:
- **[貢献ガイド](CONTRIBUTING.md)** - 開発ワークフローとPRプロセス
- **[行動規範](CODE_OF_CONDUCT.md)** - コミュニティガイドライン
- **[セキュリティポリシー](SECURITY.md)** - 脆弱性の報告

---

## 貢献者エアドロッププログラム

すべての貢献はGitHubで追跡されます。NOFXが収益を生み出すと、貢献者は貢献に基づいてエアドロップを受け取ります。

**[ピン留めされたIssue](https://github.com/NoFxAiOS/nofx/issues)を解決するPRは最高報酬を受け取ります！**

| 貢献タイプ | 重み |
|------------------|:------:|
| **ピン留めIssue PR** | ⭐⭐⭐⭐⭐⭐ |
| **コードコミット** (マージされたPR) | ⭐⭐⭐⭐⭐ |
| **バグ修正** | ⭐⭐⭐⭐ |
| **機能提案** | ⭐⭐⭐ |
| **バグ報告** | ⭐⭐ |
| **ドキュメント** | ⭐⭐ |

---

## リスク警告

1. 暗号通貨市場は非常に変動が激しい - AIの決定は利益を保証しない
2. 先物取引はレバレッジを使用 - 損失は元本を超える可能性がある
3. 極端な市場状況では清算リスクがある



## コンタクト

- **GitHub Issues**: [Issue を提出](https://github.com/NoFxAiOS/nofx/issues)
- **開発者コミュニティ**: [Telegram グループ](https://t.me/nofx_dev_community)

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
