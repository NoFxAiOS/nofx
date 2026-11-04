<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — Інтелект у дії. Відкрита торгова система на вашому комп’ютері." width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">Почати</a> ·
  <a href="#documentation">Документація</a> ·
  <a href="#community">Спільнота</a>
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
  <a href="../ru/README.md">Русский</a> ·
  <a href="README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

NOFX — торгова система з відкритим кодом, яку ви запускаєте на власному комп’ютері. У **Frontier** можна обговорити угоду з ШІ, порівняти варіанти входу на графіку й самостійно схвалити ордер. **Autopilot** дає змогу безперервно виконувати стратегію. Обидва режими використовують налаштовані вами моделі та біржові облікові записи.

Розробляється разом із [Vergex](https://vergex.trade). Доступ до ШІ та даних — через [Claw402](https://claw402.ai).

<a id="run-locally"></a>

## Локальний запуск

**Встановлення однією командою — Linux / macOS**

Встановіть і запустіть Docker із Compose, а потім виконайте:

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Відкрийте **http://127.0.0.1:3000** у браузері на комп’ютері.

Інсталятор створить відсутні ключі середовища та запустить NOFX з опублікованих Docker-образів. Встановлювати Go чи Node.js не потрібно. Перейдіть до [першого налаштування](#your-first-session) або перегляньте [збирання з вихідного коду та інші варіанти](#source-builds-and-other-options).

## Frontier

Графік, розмова та план угоди, який можна перевірити до надсилання ордера.

1. **Виберіть ринок.** Переглядайте інструменти з найбільшим обсягом торгів, шукайте за біржовими символами або користуйтеся списком Binance AI500. Інструменти, прив’язані до акцій, доступні на біржах, які їх підтримують.
2. **Порівняйте способи входу.** Запросіть ринковий, лімітний, ступінчастий план із розподілом обсягу або TWAP. Ціни входу, стоп-лос, цілі, кредитне плече, вартість позиції та розрахункова маржа відображаються разом.
3. **Перевірте й підтвердьте угоду.** Виберіть план, щоб сфокусувати на ньому графік. Перевірте поточний обліковий запис і параметри ордера, а потім явно схваліть виконання.
4. **Керуйте позицією.** Під графіком доступні поточні позиції та біржові ордери, а також ручне відкриття й закриття позицій.

Frontier підтримує **безстрокові USDT-контракти Binance, Hyperliquid, OKX і Bitget**. Публічні ринкові дані можна переглядати до підключення торгового облікового запису; для торгівлі потрібен обліковий запис відповідної біржі. Створення плану не надсилає ордер.

| Вхід | Як працює |
| :--- | :--- |
| **Ринковий** | Вхід за доступною ринковою ціною після схвалення; показана ціна — оцінка. |
| **Лімітний** | Ордер на вхід за вибраною ціною. |
| **Ступінчастий** | Обсяг позиції розподіляється між ціновими рівнями в заданих пропорціях. |
| **TWAP** | Обмежена кількість ордерів надсилається за розкладом у межах схвалених цін. |

Лімітні ордери й заплановані частини TWAP можуть залишитися невиконаними. [Докладніше про виконання та відновлення](../../features/trade-planning.md).

## Autopilot

Для безперервної роботи стратегії налаштуйте трейдера, вибравши модель, біржовий обліковий запис і стратегію. Autopilot аналізує ринок, ухвалює рішення та надсилає ордери через біржовий модуль NOFX. Dashboard показує позиції, результати й історію рішень.

**Frontier потребує схвалення кожної угоди. Autopilot може розміщувати ордери під час роботи.** Запуск і зупинка Autopilot виконуються окремо.

| Розділ | Призначення |
| :--- | :--- |
| **Config** | Підключення моделей, поповнення гаманця для ШІ та керування торговими обліковими записами. |
| **Frontier** | Дослідження ринків, порівняння планів входу й розміщення перевірених ордерів. |
| **Dashboard** | Активні трейдери, позиції та історія рішень. |
| **Strategy** | Вибір ринків, індикаторів, розміру позиції й торгових інструкцій. |
| **Leaderboard** | Порівняння опублікованих результатів трейдерів. |

<a id="your-first-session"></a>

## Перше налаштування

1. **Створіть локальний обліковий запис.** Увійдіть у щойно запущений екземпляр NOFX.
2. **Налаштуйте ШІ в Config.** Виберіть модель. Для Claw402 створіть або імпортуйте гаманець для оплати ШІ, збережіть його приватний ключ і поповніть адресу **USDC у мережі Base**. Цей баланс оплачує запити до моделей і платних даних.
3. **Додайте торговий обліковий запис.** Підключіть потрібну біржу, перевірте капітал і доступний баланс. Торгові кошти відокремлені від гаманця для оплати ШІ.
4. **Відкрийте Frontier.** Виберіть біржу, обліковий запис та інструмент. Створіть плани, порівняйте їх на графіку й перевірте ордер, коли будете готові.

Для безперервної торгівлі окремо налаштуйте й запустіть Autopilot.

### Оновлення середовища, створеного інсталятором

Зупиніть активних трейдерів Autopilot і перевірте угоди Frontier, що виконуються. Потім у каталозі встановлення (типово `~/nofx`) виконайте:

```bash
docker compose pull
docker compose up -d
```

Під час оновлення збережіть `.env` і каталог `data/`: вони містять налаштування, матеріали для шифрування та збережений стан застосунку.

Після перезапуску перевірте позиції й ордери на біржі та звірте всі виконання, що потребують уваги, перш ніж розпочинати наступну угоду. Зупинка процесу не закриває позиції.

<a id="source-builds-and-other-options"></a>

## Збирання з вихідного коду та інші варіанти

<details>
<summary>Для розробників: збирання й оновлення з вихідного коду</summary>

Цей варіант підходить для розробки NOFX або перевірки змін до оновлення опублікованих образів. Встановіть Git і Docker із Compose та запустіть Docker:

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

Скрипт запуску за потреби створює `.env` і відсутні ключі шифрування. Якщо репозиторій клоновано без підмодулів, перед збиранням виконайте `git submodule update --init web/vendor/charting`. На ресурси графіків TradingView поширюється відповідна ліцензія; див. [примітки щодо інтеграції графіків](../../features/trade-planning.md#harness-chart-and-plugins).

Щоб оновити середовище, встановлене з вихідного коду, зупиніть активних трейдерів Autopilot, перевірте угоди Frontier, що виконуються, і виконайте:

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows, Railway та віддалене розгортання</summary>

**Windows**

Використовуйте Docker Desktop і середовище Bash, наприклад WSL2, а потім виконайте наведену вище команду встановлення.

**Railway**

[![Розгорнути на Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**Віддалене розгортання**

Інсталятор використовує виробничу конфігурацію Compose, яка типово відкриває порти на всіх мережевих інтерфейсах хоста. Конфігурація Compose для вихідного коду типово прив’язує вебінтерфейс і API до `127.0.0.1`. Перед відкриттям віддаленого доступу налаштуйте адреси прив’язки та HTTPS-проксі з автентифікацією.

</details>

<details>
<summary>Розробка без Docker</summary>

Використовуйте версію Go з [go.mod](../../../go.mod), Node.js 24, як у Docker-збірці вебінтерфейсу, і TA-Lib. Перед запуском вебінтерфейсу ініціалізуйте підмодуль графіків. Налаштуйте середовище за [.env.example](../../../.env.example).

```bash
# Серверна частина
go build -o nofx .
./nofx

# Вебінтерфейс, в іншому терміналі
cd web
npm ci
npm run dev
```

</details>

## Моделі та дані

Frontier і Autopilot використовують моделі, налаштовані в NOFX. [Claw402](https://claw402.ai) надає доступ до кількох моделей з оплатою за використання в USDC у мережі Base. Виберіть модель у Config і перемикайте її в робочому просторі.

Ціни та свічки надходять із вибраної біржі. AI500 і підтримувана аналітика власників активів використовують платні маршрути Claw402. Наявність даних залежить від ринку й постачальника: дані власників активів Hyperliquid не замінюють дані Binance, OKX чи Bitget. Стрічка активності показує завантаження даних і перебіг планування.

Плата за ШІ, платні дані й торгові кошти на біржі обліковуються окремо. Запити до моделей можуть оплачуватися, навіть якщо угоду не було здійснено.

## Біржі

У Frontier зараз доступні Binance, Hyperliquid, OKX і Bitget. Загальне середовище виконання NOFX також включає Bybit, Gate, KuCoin, Aster і Lighter. Доступні продукти, типи ордерів і права облікового запису залежать від біржі.

Нижче наведено партнерські посилання для реєстрації, що підтримують розробку. Умови знижок на комісії визначає кожна біржа.

| Біржа | Підтримка | Партнерське посилання |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [Реєстрація](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [Реєстрація](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [Реєстрація](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [Реєстрація](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [Реєстрація](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [Реєстрація](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [Реєстрація](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [Реєстрація](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [Реєстрація](https://app.lighter.xyz/?referral=68151432)                              |

## Як виконуються ордери

Плани й рішення стратегій використовують наявні біржові адаптери NOFX. Приватні ключі гаманців і біржові облікові дані не передаються моделі в промпті.

- **Перевірка перед схваленням.** Frontier показує обліковий запис, розмір позиції та параметри входу й виходу. Autopilot дотримується налаштованої стратегії та обмежень середовища виконання.
- **Розмір відповідно до коштів.** Frontier використовує вибране плече й поточну доступну маржу, резервуючи витрати на виконання. Обмеження кількості, кроку ціни та біржі залишаються чинними.
- **Захисні ордери на біржі.** Після підтвердження виконання система виставляє підтримувані стоп-лоси й тейк-профіти. Вхід і захист не є однією атомарною операцією; у разі збою захисту перевірте стан на біржі.
- **Звіряння ордерів.** Невизначені результати надсилання перевіряються на біржі, а не спричиняють сліпого повторного надсилання. Невирішені виконання можуть призупинити нові входи.
- **Локальна конфігурація.** Біржові облікові дані зберігаються на сервері NOFX у зашифрованому вигляді. У вашому середовищі також є матеріали для підпису: захищайте файли та резервні копії.

## Архітектура

```text
Браузер на комп’ютері
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier: планування, перевірка та схвалення
      → Autopilot: цикл стратегії
    → Спільне виконання → Біржові адаптери → Ваші торгові облікові записи

Моделі та платні дані: клієнти провайдерів NOFX · Claw402
Сховище: локальні налаштування, плани та записи виконання
```

<details>
<summary>Огляд попереднього інтерфейсу</summary>

У відео показано попередню версію інтерфейсу. Відтоді навігація та оформлення змінилися.

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## Матеріали бренду

- [Знак логотипа — SVG](../../../web/public/icons/nofx-flow.svg)
- [Аватар GitHub — PNG, 512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [Аватар GitHub — вихідний SVG](../../../web/public/brand/nofx-avatar.svg)
- [Банер README — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## Документація

| Посібник | Зміст |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | Планування, реальне виконання, дані та відновлення |
| [Початок роботи](../../getting-started/README.md) | Розгортання та API бірж |
| [Архітектура](../../architecture/README.md) | Устрій системи й перелік модулів |
| [Модуль стратегій](../../architecture/STRATEGY_MODULE.md) | Вибір монет, промпти ШІ та виконання |
| [Поширені запитання](../../guides/faq.en.md) | Відповіді на типові запитання |
| [Усунення несправностей](../../guides/TROUBLESHOOTING.md) | Діагностика типових проблем |

<a id="community"></a>

## Спільнота

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [Живий дашборд](https://vergex.trade/explore)

## Участь у розробці

Код, документація, переклади та звіти про помилки — усе це вітається; див. [Посібник для контриб'юторів](../../../CONTRIBUTING.md), [Кодекс поведінки](../../../CODE_OF_CONDUCT.md) і [Політику безпеки](../../../SECURITY.md).

NOFX відстежує значущі внески й має намір винагороджувати контриб'юторів у міру зростання екосистеми. Пріоритетні issues мають більшу вагу.

| Внесок            | Вага |
| :---------------- | :----: |
| PR до закріплених issues | ★★★★★★ |
| Код (змерджені PR) | ★★★★★  |
| Виправлення багів |  ★★★★  |
| Ідеї функцій      |  ★★★   |
| Звіти про баги    |   ★★   |
| Документація      |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

## Спонсори

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

[Стати спонсором](https://github.com/sponsors/NoFxAiOS)

<br/>

Якщо NOFX вам корисний, зірка допоможе іншим трейдерам його знайти.

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## Ліцензія

[AGPL-3.0](../../../LICENSE)

<sub>Автоматизована торгівля пов'язана зі значним ризиком. Стратегії на основі AI експериментальні й можуть втрачати гроші. Обирайте розмір позицій розважливо, розумійте кожен майданчик і ніколи не торгуйте коштами, втрату яких не можете собі дозволити. Повний [дисклеймер](../../../DISCLAIMER.md).</sub>
