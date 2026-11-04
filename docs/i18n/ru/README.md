<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — Интеллект в действии. Открытая торговая система на вашем компьютере." width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">Начать</a> ·
  <a href="#documentation">Документация</a> ·
  <a href="#community">Сообщество</a>
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
  <a href="../ko/README.md">한국어</a> ·
  <a href="README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

NOFX — торговая система с открытым исходным кодом, которую вы запускаете на своём компьютере. В **Frontier** можно обсудить сделку с ИИ, сравнить варианты входа на графике и самостоятельно одобрить ордер. **Autopilot** предназначен для непрерывной работы стратегии. Оба режима используют настроенные вами модели и биржевые аккаунты.

Разрабатывается совместно с [Vergex](https://vergex.trade). Доступ к ИИ и данным — через [Claw402](https://claw402.ai).

<a id="run-locally"></a>

## Локальный запуск

**Установка одной командой — Linux / macOS**

Установите и запустите Docker с Compose, затем выполните:

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Откройте **http://127.0.0.1:3000** в браузере на компьютере.

Установщик создаст недостающие ключи окружения и запустит NOFX из опубликованных Docker-образов. Устанавливать Go или Node.js не требуется. Перейдите к [первой настройке](#your-first-session) или посмотрите [сборку из исходников и другие варианты](#source-builds-and-other-options).

## Frontier

График, диалог и план сделки, который можно проверить до отправки ордера.

1. **Выберите рынок.** Просматривайте инструменты с наибольшим объёмом, ищите по биржевым тикерам или используйте список Binance AI500. Инструменты, привязанные к акциям, доступны на поддерживающих их площадках.
2. **Сравните варианты входа.** Запросите рыночный, лимитный, ступенчатый план с распределением объёма или TWAP. Цены входа, стоп-лосс, цели, плечо, стоимость позиции и расчётная маржа показаны вместе.
3. **Проверьте и подтвердите сделку.** Выберите план, чтобы сфокусировать на нём график. Проверьте текущий аккаунт и параметры ордера, затем явно одобрите исполнение.
4. **Управляйте позицией.** Под графиком доступны текущие позиции и биржевые ордера, а также ручное открытие и закрытие позиций.

Frontier поддерживает **бессрочные USDT-контракты Binance, Hyperliquid, OKX и Bitget**. Публичные рыночные данные можно просматривать до подключения торгового аккаунта; для торговли нужен аккаунт соответствующей биржи. Создание плана не отправляет ордер.

| Вход | Как работает |
| :--- | :--- |
| **Рыночный** | Вход по доступной рыночной цене после одобрения; показанная цена — оценка. |
| **Лимитный** | Ордер на вход по выбранной цене. |
| **Ступенчатый** | Объём позиции распределяется между ценовыми уровнями в заданных пропорциях. |
| **TWAP** | Ограниченное число ордеров отправляется по времени в пределах одобренных цен. |

Лимитные ордера и запланированные части TWAP могут остаться неисполненными. [Подробности исполнения и восстановления](../../features/trade-planning.md).

## Autopilot

Для непрерывной работы стратегии настройте трейдера, выбрав модель, биржевой аккаунт и стратегию. Autopilot анализирует рынок, принимает решения и отправляет ордера через биржевой модуль NOFX. Dashboard показывает позиции, результаты и историю решений.

**Frontier требует одобрения каждой сделки. Autopilot может размещать ордера во время работы.** Запуск и остановка Autopilot выполняются отдельно.

| Раздел | Назначение |
| :--- | :--- |
| **Config** | Подключение моделей, пополнение AI-кошелька и управление торговыми аккаунтами. |
| **Frontier** | Изучение рынков, сравнение планов входа и размещение проверенных ордеров. |
| **Dashboard** | Работающие трейдеры, позиции и история решений. |
| **Strategy** | Выбор рынков, индикаторов, размера позиции и торговых инструкций. |
| **Leaderboard** | Сравнение опубликованных результатов трейдеров. |

<a id="your-first-session"></a>

## Первая настройка

1. **Создайте локальную учётную запись.** Войдите в запущенный экземпляр NOFX.
2. **Настройте ИИ в Config.** Выберите модель. Для Claw402 создайте или импортируйте кошелёк для оплаты ИИ, сохраните его приватный ключ и пополните адрес **USDC в сети Base**. Этот баланс оплачивает запросы к моделям и платным данным.
3. **Добавьте торговый аккаунт.** Подключите нужную биржу и проверьте капитал и доступный баланс аккаунта. Торговые средства отделены от кошелька для оплаты ИИ.
4. **Откройте Frontier.** Выберите биржу, аккаунт и инструмент. Создайте планы, сравните их на графике и, когда будете готовы, проверьте ордер.

Для непрерывной торговли отдельно настройте и запустите Autopilot.

### Обновление установки, созданной установщиком

Остановите активных трейдеров Autopilot и проверьте выполняющиеся сделки Frontier. Затем в каталоге установки (по умолчанию `~/nofx`) выполните:

```bash
docker compose pull
docker compose up -d
```

При обновлении сохраните `.env` и каталог `data/`: в них находятся настройки, материалы для шифрования и сохранённое состояние приложения.

После перезапуска проверьте позиции и ордера на бирже и разберите все исполнения, требующие внимания, прежде чем открывать следующую сделку. Остановка процесса не закрывает позиции.

<a id="source-builds-and-other-options"></a>

## Сборка из исходников и другие варианты

<details>
<summary>Для разработчиков: сборка и обновление из исходников</summary>

Этот вариант подходит для разработки NOFX или проверки изменений до обновления опубликованных образов. Установите Git и Docker с Compose, затем запустите Docker:

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

Скрипт запуска при необходимости создаёт `.env` и недостающие ключи шифрования. Если репозиторий клонирован без подмодулей, перед сборкой выполните `git submodule update --init web/vendor/charting`. На ресурсы графиков TradingView распространяется соответствующая лицензия; см. [примечания об интеграции графиков](../../features/trade-planning.md#harness-chart-and-plugins).

Чтобы обновить установку из исходников, остановите активных трейдеров Autopilot, проверьте выполняющиеся сделки Frontier и выполните:

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows, Railway и удалённое развёртывание</summary>

**Windows**

Используйте Docker Desktop и среду Bash, например WSL2, затем выполните команду установки выше.

**Railway**

[![Развернуть на Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**Удалённое развёртывание**

Установщик использует производственную конфигурацию Compose, которая по умолчанию публикует порты на всех сетевых интерфейсах хоста. Конфигурация Compose для исходников по умолчанию привязывает веб-интерфейс и API к `127.0.0.1`. Перед открытием удалённого доступа настройте адреса привязки и HTTPS-прокси с аутентификацией.

</details>

<details>
<summary>Разработка без Docker</summary>

Используйте версию Go из [go.mod](../../../go.mod), Node.js 24, как в Docker-сборке веб-интерфейса, и TA-Lib. Перед запуском веб-интерфейса инициализируйте подмодуль графиков. Настройте окружение по [.env.example](../../../.env.example).

```bash
# Серверная часть
go build -o nofx .
./nofx

# Веб-интерфейс, в другом терминале
cd web
npm ci
npm run dev
```

</details>

## Модели и данные

Frontier и Autopilot используют модели, настроенные в NOFX. [Claw402](https://claw402.ai) предоставляет доступ к нескольким моделям с оплатой по использованию в USDC в сети Base. Выберите модель в Config и переключайте её в рабочем пространстве.

Цены и свечи поступают с выбранной биржи. AI500 и поддерживаемая аналитика держателей используют платные маршруты Claw402. Наличие данных зависит от рынка и поставщика: данные держателей Hyperliquid не заменяют данные Binance, OKX или Bitget. Лента активности показывает загрузку данных и ход планирования.

Плата за ИИ, платные данные и торговые средства на бирже учитываются отдельно. Запросы к моделям могут оплачиваться, даже если сделка не была совершена.

## Биржи

В Frontier сейчас доступны Binance, Hyperliquid, OKX и Bitget. Общая среда исполнения NOFX также включает Bybit, Gate, KuCoin, Aster и Lighter. Доступные продукты, типы ордеров и права аккаунта зависят от биржи.

Ниже приведены партнёрские ссылки для регистрации, поддерживающие разработку. Условия скидок на комиссии определяет каждая биржа.

| Биржа | Поддержка | Партнёрская ссылка |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [Регистрация](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [Регистрация](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [Регистрация](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [Регистрация](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [Регистрация](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [Регистрация](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [Регистрация](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [Регистрация](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [Регистрация](https://app.lighter.xyz/?referral=68151432)                              |

## Как исполняются ордера

Планы и решения стратегий используют существующие биржевые адаптеры NOFX. Приватные ключи кошельков и биржевые учётные данные не передаются модели в промпте.

- **Проверка перед одобрением.** Frontier показывает аккаунт, размер позиции, параметры входа и выхода. Autopilot следует настроенной стратегии и ограничениям среды исполнения.
- **Размер с учётом аккаунта.** Frontier использует выбранное плечо и текущую доступную маржу, резервируя затраты на исполнение. Ограничения количества, шага цены и биржи продолжают действовать.
- **Защитные ордера на бирже.** По мере подтверждения исполнений система выставляет поддерживаемые стоп-лоссы и тейк-профиты. Вход и защита не являются одной атомарной операцией; при сбое защиты проверьте состояние на бирже.
- **Сверка ордеров.** Неопределённые результаты отправки проверяются на бирже, а не приводят к слепой повторной отправке. Неразрешённые исполнения могут приостановить новые входы.
- **Локальная конфигурация.** Биржевые учётные данные хранятся на сервере NOFX в зашифрованном виде. В вашей установке также находятся материалы для подписи: защищайте файлы и резервные копии.

## Архитектура

```text
Браузер на компьютере
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier: планирование, проверка и одобрение
      → Autopilot: цикл стратегии
    → Общее исполнение → Биржевые адаптеры → Ваши торговые аккаунты

Модели и платные данные: клиенты провайдеров NOFX · Claw402
Хранилище: локальные настройки, планы и записи исполнения
```

<details>
<summary>Обзор прежнего интерфейса</summary>

В видео показана более ранняя версия интерфейса. С тех пор навигация и оформление изменились.

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## Материалы бренда

- [Знак логотипа — SVG](../../../web/public/icons/nofx-flow.svg)
- [Аватар GitHub — PNG, 512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [Аватар GitHub — исходник SVG](../../../web/public/brand/nofx-avatar.svg)
- [Баннер README — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## Документация

| Руководство | Содержание |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | Планирование, реальное исполнение, данные и восстановление |
| [Начало работы](../../getting-started/README.md) | Развёртывание и API бирж |
| [Архитектура](../../architecture/README.md) | Устройство системы и перечень модулей |
| [Модуль стратегий](../../architecture/STRATEGY_MODULE.md) | Выбор монет, промпты ИИ и исполнение |
| [Частые вопросы](../../guides/faq.en.md) | Ответы на распространённые вопросы |
| [Устранение неполадок](../../guides/TROUBLESHOOTING.md) | Диагностика типичных проблем |

<a id="community"></a>

## Сообщество

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [Живой дашборд](https://vergex.trade/explore)

## Участие в разработке

Код, документация, переводы и сообщения об ошибках — всё это приветствуется; см. [руководство для контрибьюторов](../../../CONTRIBUTING.md), [кодекс поведения](../../../CODE_OF_CONDUCT.md) и [политику безопасности](../../../SECURITY.md).

NOFX отслеживает значимые вклады и намерен вознаграждать контрибьюторов по мере роста экосистемы. Приоритетные issues имеют больший вес.

| Вклад      | Вес |
| :---------------- | :----: |
| PR по закреплённым issues  | ★★★★★★ |
| Код (принятые PR) | ★★★★★  |
| Исправления ошибок         |  ★★★★  |
| Идеи новых функций     |  ★★★   |
| Сообщения об ошибках       |   ★★   |
| Документация     |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

## Спонсоры

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

[Стать спонсором](https://github.com/sponsors/NoFxAiOS)

<br/>

Если NOFX вам полезен, звезда помогает другим трейдерам его найти.

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## Лицензия

[AGPL-3.0](../../../LICENSE)

<sub>Автоматическая торговля сопряжена со значительным риском. Стратегии на основе AI экспериментальны и могут терять деньги. Разумно выбирайте размер позиций, разбирайтесь в устройстве каждой площадки и никогда не торгуйте средствами, потерю которых не можете себе позволить. Полный [дисклеймер](../../../DISCLAIMER.md).</sub>
