import { Brain, Landmark, Rocket, Sparkles } from 'lucide-react'

interface BeginnerGuideCardsProps {
  language: string
  claw402Ready: boolean
  exchangeReady: boolean
  strategyReady: boolean
  traderReady: boolean
  canCreateTrader: boolean
  walletAddress?: string | null
  onQuickSetupClaw402: () => void
  onOpenExchange: () => void
  onOpenStrategy: () => void
  onCreateTrader: () => void
}

function truncateAddress(address: string) {
  if (address.length <= 12) return address
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

export function BeginnerGuideCards({
  language,
  claw402Ready,
  exchangeReady,
  strategyReady,
  traderReady,
  canCreateTrader,
  walletAddress,
  onQuickSetupClaw402,
  onOpenExchange,
  onOpenStrategy,
  onCreateTrader,
}: BeginnerGuideCardsProps) {
  const isZh = language === 'zh'

  const cards = [
    {
      key: 'model',
      icon: Brain,
      title: isZh ? '1. 极速模型' : '1. Fast AI',
      desc: isZh
        ? '默认就是 Claw402 + DeepSeek。第一次不用挑模型，先跑起来。'
        : 'Start with Claw402 + DeepSeek. No model picking needed for the first run.',
      meta: walletAddress
        ? isZh
          ? `钱包 ${truncateAddress(walletAddress)}`
          : `Wallet ${truncateAddress(walletAddress)}`
        : isZh
          ? 'Base 链 USDC 按次付费'
          : 'Pay per call with Base USDC',
      ready: claw402Ready,
      actionLabel: claw402Ready
        ? isZh
          ? '已配置'
          : 'Configured'
        : isZh
          ? '一键配置'
          : 'One-click setup',
      onAction: onQuickSetupClaw402,
      disabled: claw402Ready,
    },
    {
      key: 'exchange',
      icon: Landmark,
      title: isZh ? '2. 连接交易所' : '2. Add Exchange',
      desc: isZh
        ? '交易所接好以后，AI 才能真正下单。'
        : 'Connect an exchange so the AI can actually place trades.',
      meta: exchangeReady
        ? isZh
          ? '已准备好'
          : 'Ready'
        : isZh
          ? 'Binance / OKX / Bybit / Hyperliquid'
          : 'Binance / OKX / Bybit / Hyperliquid',
      ready: exchangeReady,
      actionLabel: exchangeReady
        ? isZh
          ? '继续管理'
          : 'Manage'
        : isZh
          ? '去配置'
          : 'Configure',
      onAction: onOpenExchange,
      disabled: false,
    },
    {
      key: 'strategy',
      icon: Sparkles,
      title: isZh ? '3. 选择策略' : '3. Pick Strategy',
      desc: isZh
        ? '先用默认策略也可以，后面再慢慢细调。'
        : 'You can start with a default strategy and fine-tune later.',
      meta: strategyReady
        ? isZh
          ? '已有策略可用'
          : 'Strategy ready'
        : isZh
          ? '可选，但建议提前看一眼'
          : 'Optional, but worth a quick look',
      ready: strategyReady,
      actionLabel: isZh ? '打开策略页' : 'Open strategy',
      onAction: onOpenStrategy,
      disabled: false,
    },
    {
      key: 'trader',
      icon: Rocket,
      title: isZh ? '4. 创建 Trader' : '4. Create Trader',
      desc: isZh
        ? '最后一步，把模型和交易所绑在一起，就能开始运行。'
        : 'Last step: bind your model and exchange, then start running.',
      meta: traderReady
        ? isZh
          ? '已创建 Trader，可继续添加'
          : 'Trader created, you can add more'
        : canCreateTrader
          ? isZh
            ? '已经可以创建'
            : 'Ready to create'
        : isZh
          ? '先完成前三步'
          : 'Finish the first three steps first',
      ready: traderReady,
      actionLabel: traderReady
        ? isZh
          ? '继续创建'
          : 'Create another'
        : isZh
          ? '立即创建'
          : 'Create now',
      onAction: onCreateTrader,
      disabled: !canCreateTrader,
    },
  ]

  return (
    <section className="space-y-4 rounded-[28px] border border-white/10 bg-zinc-950/60 p-5 backdrop-blur-xl">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="text-xs font-semibold uppercase tracking-[0.3em] text-nofx-gold/80">
            {isZh ? '新手引导' : 'Quickstart'}
          </div>
          <h2 className="mt-1 text-xl font-bold text-white">
            {isZh
              ? '先按这 4 步走，最快上手'
              : 'Follow these 4 steps to get started fast'}
          </h2>
        </div>
        {/* <div className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-zinc-400">
          {isZh ? '老手模式不会看到这块' : 'Hidden in advanced mode'}
        </div> */}
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {cards.map((card) => {
          const Icon = card.icon
          return (
            <div
              key={card.key}
              className="rounded-[22px] border border-white/8 bg-black/25 p-4"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/6 text-nofx-gold">
                  <Icon className="h-5 w-5" />
                </div>
                <span
                  className={`rounded-full px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.22em] ${
                    card.ready
                      ? 'bg-emerald-500/15 text-emerald-300'
                      : 'bg-zinc-800 text-zinc-400'
                  }`}
                >
                  {card.ready
                    ? isZh
                      ? '已就绪'
                      : 'Ready'
                    : isZh
                      ? '待完成'
                      : 'Pending'}
                </span>
              </div>

              <h3 className="mt-4 text-base font-semibold text-white">
                {card.title}
              </h3>
              <p className="mt-2 min-h-[72px] text-sm leading-6 text-zinc-400">
                {card.desc}
              </p>
              <div className="mt-3 text-xs text-zinc-500">{card.meta}</div>

              <button
                type="button"
                onClick={card.onAction}
                disabled={card.disabled}
                className={`mt-5 w-full rounded-2xl px-4 py-3 text-sm font-semibold transition ${
                  card.disabled
                    ? 'cursor-not-allowed bg-zinc-900 text-zinc-500'
                    : 'bg-nofx-gold text-black hover:bg-yellow-400'
                }`}
              >
                {card.actionLabel}
              </button>
            </div>
          )
        })}
      </div>
    </section>
  )
}
