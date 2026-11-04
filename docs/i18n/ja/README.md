<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — 取引を動かすインテリジェンス。自分で運用するオープンな取引システム。" width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">はじめる</a> ·
  <a href="#documentation">ドキュメント</a> ·
  <a href="#community">コミュニティ</a>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=flat&labelColor=192434&color=295BE8" alt="GitHub stars"></a>
  <a href="../../../LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-295BE8?style=flat&labelColor=192434" alt="License: AGPL-3.0"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/telegram-community-295BE8?style=flat&labelColor=192434&logo=telegram&logoColor=white" alt="Telegram community"></a>
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

NOFX は、自分のコンピューターで動かすオープンソースの取引システムです。**Frontier** では AI と取引のアイデアを検討し、チャート上でエントリー案を比較して、自分で注文を承認できます。**Autopilot** では戦略を継続的に実行できます。どちらも、設定済みのモデルと取引所口座を利用します。

[Vergex](https://vergex.trade) とともに開発。AI とデータへのアクセスは [Claw402](https://claw402.ai) が提供します。

<a id="run-locally"></a>

## ローカルで実行

**1 行でインストール — Linux / macOS**

Compose を含む Docker をインストールして起動し、次のコマンドを実行します。

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

デスクトップのブラウザーで **http://127.0.0.1:3000** を開いてください。

インストーラーは不足している環境設定用の鍵を生成し、公開済みの Docker イメージで NOFX を起動します。Go や Node.js のインストールは不要です。[初回の設定](#your-first-session)へ進むか、[ソースからのビルドとその他の方法](#source-builds-and-other-options)を確認してください。

## Frontier

チャートと対話を通じて、発注前に内容を確認できる取引プランを作ります。

1. **市場を選ぶ。** 出来高上位の銘柄を探す、取引所のシンボルで検索する、または Binance AI500 リストを利用できます。取引所が対応していれば、株式連動銘柄も表示されます。
2. **エントリー方法を比較する。** 成行、指値、配分比率付きの段階注文、TWAP のプランを依頼できます。エントリー価格、損切り、目標、レバレッジ、ポジション金額、推定証拠金をまとめて確認できます。
3. **確認して発注する。** プランを選ぶとチャートがそのプランにフォーカスします。現在の口座と注文内容を確認してから、実行を明示的に承認します。
4. **ポジションを管理する。** チャートの下で実際のポジションと取引所の注文を確認し、手動の新規注文や決済も行えます。

Frontier は **Binance の USDT 無期限先物、Hyperliquid、OKX、Bitget** に対応しています。取引用口座を接続する前でも公開市場データを閲覧できますが、取引には対応する口座が必要です。プランの生成だけでは注文は送信されません。

| エントリー方法 | 動作 |
| :--- | :--- |
| **成行** | 承認後、約定可能な市場価格でエントリーします。表示価格は推定値です。 |
| **指値** | 指定した価格でエントリー注文を出します。 |
| **段階注文** | 複数の価格帯へ、指定の比率でポジションを分割します。 |
| **TWAP** | 承認された価格範囲内で、決められた回数の注文を時間で分割して送信します。 |

指値注文や時間分割注文は約定しない場合があります。[実行と復旧の詳細](../../features/trade-planning.md)を参照してください。

## Autopilot

戦略を継続的に動かすには、モデル、取引所口座、戦略を指定してトレーダーを設定します。Autopilot は市場を読み取り、判断を行い、NOFX の取引実行基盤を通じて注文します。Dashboard でポジション、運用成績、判断履歴を確認できます。

**Frontier では取引ごとに承認が必要です。Autopilot は稼働中に自動で注文できます。** Autopilot の開始と停止は、それぞれ別の操作です。

| ワークスペース | 用途 |
| :--- | :--- |
| **Config** | モデルの接続、AI ウォレットへの入金、取引口座の管理。 |
| **Frontier** | 市場の探索、エントリー案の比較、内容を確認した注文の発注。 |
| **Dashboard** | 稼働中のトレーダー、ポジション、判断履歴の確認。 |
| **Strategy** | 銘柄選定、指標、ポジションサイズ、取引指示の設定。 |
| **Leaderboard** | 公開されている運用成績の比較。 |

<a id="your-first-session"></a>

## 初回の設定

1. **ローカルアカウントを作成する。** 起動した NOFX にサインインします。
2. **Config で AI を設定する。** モデルを選びます。Claw402 を使う場合は、AI 利用料用のウォレットを作成またはインポートし、秘密鍵を保存して、**Base 上の USDC** をそのアドレスに送金します。この残高からモデルと有料データのリクエスト料金を支払います。
3. **取引口座を追加する。** 利用する取引所を接続し、口座資産と利用可能残高を確認します。取引資金と AI 利用料用ウォレットは別々です。
4. **Frontier を開く。** 取引所、口座、銘柄を選択します。プランを生成してチャート上で比較し、準備ができたら注文内容を確認します。

継続的な自動取引を行う場合は、Autopilot を別途設定して起動してください。

### インストーラーで導入した環境の更新

稼働中の Autopilot を停止し、実行中の Frontier の取引状況を確認してから、インストール先のディレクトリ（既定では `~/nofx`）で実行します。

```bash
docker compose pull
docker compose up -d
```

更新時も `.env` と `data/` ディレクトリを保持してください。設定、暗号化に必要な情報、保存済みのアプリケーション状態が含まれています。

再起動後は取引所でポジションと注文を確認し、要確認となっている実行記録の照合を済ませてから次の取引を行ってください。プロセスを停止してもポジションは決済されません。

<a id="source-builds-and-other-options"></a>

## ソースからのビルドとその他の方法

<details>
<summary>開発者向け：ソースからビルド・更新する</summary>

NOFX の開発や、公開イメージに反映される前の変更を試す場合に利用します。Git と Compose を含む Docker をインストールし、Docker を起動してください。

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

起動スクリプトは必要に応じて `.env` を作成し、不足している暗号化用の鍵を生成します。サブモジュールなしでクローンした場合は、ビルド前に `git submodule update --init web/vendor/charting` を実行してください。TradingView のチャート素材には該当するライセンスが適用されます。[チャート統合の説明](../../features/trade-planning.md#harness-chart-and-plugins)を参照してください。

ソースから導入した環境を更新する場合は、稼働中の Autopilot を停止し、実行中の Frontier の取引状況を確認してから実行します。

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows、Railway、リモート環境へのデプロイ</summary>

**Windows**

Docker Desktop と WSL2 などの Bash 環境を用意し、上記の 1 行インストールコマンドを実行します。

**Railway**

[![Railway にデプロイ](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**リモート環境**

インストーラーは本番用の Compose 設定を使用し、既定ではホストのすべてのネットワークインターフェースでポートを公開します。ソース側の Compose 設定は、フロントエンドと API を既定で `127.0.0.1` にバインドします。リモートのインスタンスを公開する前に、バインド先と認証付き HTTPS プロキシを設定してください。

</details>

<details>
<summary>Docker を使わずに開発する</summary>

[go.mod](../../../go.mod) に指定された Go、フロントエンドの Docker ビルドと同じ Node.js 24、および TA-Lib を使用します。フロントエンドの起動前にチャートのサブモジュールを初期化し、[.env.example](../../../.env.example) を参考に環境を設定してください。

```bash
# バックエンド
go build -o nofx .
./nofx

# フロントエンド：別のターミナルで実行
cd web
npm ci
npm run dev
```

</details>

## モデルとデータ

Frontier と Autopilot は NOFX に設定されたモデルを共用します。[Claw402](https://claw402.ai) は複数のモデルへのアクセスを提供し、Base 上の USDC で従量精算します。Config でモデルを選び、ワークスペース内で切り替えられます。

価格とローソク足は選択した取引所から取得します。AI500 と対応する保有者分析には Claw402 の有料経路を使用します。利用できるデータは市場と提供元によって異なり、Hyperliquid の保有者データを Binance、OKX、Bitget のデータとして代用することはありません。アクティビティにはデータの取得状況とプラン作成の進捗が表示されます。

AI 利用料、有料データの料金、取引所の取引資金は別々です。取引を行わなくても、モデルへのリクエストには料金が発生する場合があります。

## 取引所

Frontier で現在選べる取引所は Binance、Hyperliquid、OKX、Bitget です。NOFX の実行基盤には、Bybit、Gate、KuCoin、Aster、Lighter も含まれます。利用できる商品、注文種別、口座権限は取引所によって異なります。

以下は開発を支援するパートナー登録リンクです。手数料の優遇条件は各取引所が定めます。

| 取引所 | 対応状況 | パートナーリンク |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [登録](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [登録](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [登録](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [登録](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [登録](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [登録](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [登録](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [登録](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [登録](https://app.lighter.xyz/?referral=68151432)                              |

## 注文の実行について

プランと戦略の判断は、既存の NOFX 取引所アダプターを通じて実行されます。モデルのプロンプトにウォレットの秘密鍵や取引所の認証情報は含まれません。

- **承認前に確認できる注文。** Frontier は口座、注文サイズ、エントリーと決済の詳細を表示します。Autopilot は設定された戦略と実行時の制限に従います。
- **口座に応じたサイズ計算。** Frontier は現在の利用可能証拠金と選択したレバレッジを使い、実行コストを確保します。数量精度、価格刻み、取引所の制限も適用されます。
- **取引所側の損切り・利確。** 約定を確認すると、対応するストップロスとテイクプロフィットの注文を出します。新規注文と保護注文は一括で確定する処理ではありません。保護注文に失敗した場合は取引所で確認してください。
- **注文状態の照合。** 送信結果が不明な場合は、無条件に再送せず取引所で照合します。未解決の実行があると、その後のエントリーが一時停止する場合があります。
- **ローカルの設定。** 取引所の認証情報は NOFX サーバーで暗号化して保存します。署名に必要な情報もインストール先に保持するため、ファイルとバックアップを保護してください。

## アーキテクチャ

```text
デスクトップブラウザー
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier：プラン作成、確認、承認
      → Autopilot：戦略ループ
    → 共通の実行基盤 → 取引所アダプター → 自分の取引口座

モデルと有料データ：NOFX プロバイダークライアント · Claw402
保存先：ローカルの設定、プラン、実行記録
```

<details>
<summary>旧インターフェースのデモ</summary>

この動画は以前のインターフェースを紹介しています。現在はナビゲーションとデザインが変更されています。

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## ブランド素材

- [シンボル — SVG](../../../web/public/icons/nofx-flow.svg)
- [GitHub アバター — PNG、512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [GitHub アバター — SVG ソース](../../../web/public/brand/nofx-avatar.svg)
- [README バナー — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## ドキュメント

| ガイド | 内容 |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | プラン作成、実取引、データ、復旧 |
| [はじめに](../../getting-started/README.md) | デプロイと取引所 API のガイド |
| [アーキテクチャ](../../architecture/README.md) | システム設計とモジュール一覧 |
| [戦略モジュール](../../architecture/STRATEGY_MODULE.md) | 銘柄選定、AI プロンプト、実行 |
| [よくある質問](../../guides/faq.en.md) | 一般的な質問への回答 |
| [トラブルシューティング](../../guides/TROUBLESHOOTING.md) | よくある問題の診断 |

<a id="community"></a>

## コミュニティ

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [ライブダッシュボード](https://vergex.trade/explore)

## コントリビューション

コード、ドキュメント、翻訳、バグ報告のいずれも歓迎します。詳細は[貢献ガイド](../../../CONTRIBUTING.md)、[行動規範](../../../CODE_OF_CONDUCT.md)、[セキュリティポリシー](../../../SECURITY.md)をご覧ください。

NOFX は有意義な貢献を記録しており、エコシステムの成長に応じて貢献者に還元していく予定です。優先度の高い Issue にはより高いウェイトが付きます。

| 貢献の種類      | ウェイト |
| :---------------- | :----: |
| ピン留め Issue の PR  | ★★★★★★ |
| コード（マージ済み PR） | ★★★★★  |
| バグ修正         |  ★★★★  |
| 機能アイデア     |  ★★★   |
| バグ報告       |   ★★   |
| ドキュメント     |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

## スポンサー

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

[スポンサーになる](https://github.com/sponsors/NoFxAiOS)

<br/>

NOFX が役に立ったら、スターを付けていただけると、他のトレーダーがこのプロジェクトを見つけやすくなります。

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## ライセンス

[AGPL-3.0](../../../LICENSE)

<sub>自動売買には大きなリスクが伴います。AI 駆動の戦略は実験的なものであり、損失を出す可能性があります。適切なポジションサイズを守り、各取引所の仕組みを理解し、失っても差し支えのない資金以外では決して取引しないでください。詳細は[免責事項](../../../DISCLAIMER.md)をご覧ください。</sub>
