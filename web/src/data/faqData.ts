import {
  BookOpen,
  GitBranch,
  Monitor,
  Shield,
  TrendingUp,
  Wrench,
  Zap,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

/**
 * FAQ content model. Answers are composed from typed blocks so the renderer
 * stays generic — no per-question JSX special cases. Inline `code` spans are
 * written with backticks and parsed by the renderer.
 */
export type FAQBlock =
  | { type: 'p'; text: string }
  | { type: 'list'; items: string[] }
  | { type: 'steps'; items: string[] }
  | { type: 'note'; text: string }
  | { type: 'links'; links: { label: string; href: string }[] }

export interface FAQItem {
  id: string
  question: string
  blocks: FAQBlock[]
}

export interface FAQCategory {
  id: string
  title: string
  icon: LucideIcon
  items: FAQItem[]
}

/** Plain text of an item, used by the search filter. */
export function faqItemSearchText(item: FAQItem): string {
  const parts: string[] = [item.question]
  for (const block of item.blocks) {
    if (block.type === 'p' || block.type === 'note') parts.push(block.text)
    else if (block.type === 'list' || block.type === 'steps')
      parts.push(block.items.join(' '))
    else if (block.type === 'links')
      parts.push(block.links.map((l) => l.label).join(' '))
  }
  return parts.join(' ').toLowerCase()
}

export const faqCategories: FAQCategory[] = [
  // ───────────────────────── Getting started ─────────────────────────
  {
    id: 'getting-started',
    title: 'Getting Started',
    icon: BookOpen,
    items: [
      {
        id: 'what-is-nofx',
        question: 'What is NOFX?',
        blocks: [
          {
            type: 'p',
            text: 'NOFX is an open-source, self-hosted AI trading terminal. Its flagship mode is the NOFX Autopilot: an AI agent that reads the Claw402.ai signal board, verifies candidates with Signal Lab and liquidation structure, confirms timing with raw candles, and executes on Hyperliquid — all on your own machine, with your keys never leaving your server.',
          },
          {
            type: 'p',
            text: 'Beyond the Autopilot you can build custom strategies in Strategy Studio, run multiple AI traders side by side, and compare them on the leaderboard.',
          },
        ],
      },
      {
        id: 'what-do-i-need',
        question: 'What do I need before launching the Autopilot?',
        blocks: [
          {
            type: 'p',
            text: 'Two funded accounts — the guided launch on the Config page walks you through both:',
          },
          {
            type: 'list',
            items: [
              'An AI fee wallet: a Base-chain USDC wallet that pays for AI model and market-data calls. Minimum `1 USDC` to launch.',
              'A Hyperliquid account with trading authorization and at least `12 USDC` available as margin.',
            ],
          },
          {
            type: 'p',
            text: 'The launch button runs a server-side preflight that checks every prerequisite and points you at the exact step that is missing, so you cannot start a half-configured bot.',
          },
        ],
      },
      {
        id: 'which-markets',
        question: 'Which markets can it trade?',
        blocks: [
          {
            type: 'p',
            text: 'The Autopilot trades Hyperliquid perpetuals: crypto majors (BTC, ETH, SOL, …) plus the xyz synthetic markets covering US stocks, indices, commodities, and FX — so one account gives the AI a multi-asset universe.',
          },
          {
            type: 'p',
            text: 'Manual traders built in Strategy Studio can also connect Binance, Bybit, OKX, Bitget, KuCoin, Gate, Aster, and Lighter.',
          },
        ],
      },
      {
        id: 'ai-models',
        question: 'Which AI models does it use? Do I need API keys?',
        blocks: [
          {
            type: 'p',
            text: 'No API keys are required. NOFX routes inference through Claw402 pay-as-you-go infrastructure: your AI fee wallet pays per call with Base USDC, and the terminal accesses supported models (DeepSeek and other frontier models) on demand.',
          },
          {
            type: 'p',
            text: 'Power users can still plug in their own provider keys (OpenAI, Claude, Gemini, DeepSeek, Qwen, Grok, Kimi, or any OpenAI-compatible endpoint) under Config → Models.',
          },
        ],
      },
      {
        id: 'is-it-profitable',
        question: 'Will it make money?',
        blocks: [
          {
            type: 'p',
            text: 'No one can promise that, and you should distrust anyone who does. The AI trades a systematic process, but markets are adversarial and past performance never guarantees future results.',
          },
          {
            type: 'p',
            text: 'The dashboard is deliberately honest about performance: it separates realized from unrealized P/L, shows the fee-drag chain (gross − fees = net), profit factor, and max drawdown computed from your real starting balance. Watch those numbers, start small, and only trade money you can afford to lose.',
          },
          {
            type: 'note',
            text: 'Trading involves substantial risk of loss. NOFX is software, not investment advice.',
          },
        ],
      },
    ],
  },

  // ───────────────────────── Launch & wallets ─────────────────────────
  {
    id: 'launch-wallets',
    title: 'Launch & Wallets',
    icon: Zap,
    items: [
      {
        id: 'ai-fee-wallet',
        question: 'What is the AI fee wallet?',
        blocks: [
          {
            type: 'p',
            text: 'A dedicated EVM wallet on Base that pays for AI model calls and paid market data (x402 micropayments). It is completely separate from your trading collateral — it never touches Hyperliquid.',
          },
          {
            type: 'list',
            items: [
              'The guided setup creates it for you (or reuses an existing one).',
              'Deposit only USDC on the Base network to its address.',
              'Launch requires at least `1 USDC`; the balance display refreshes automatically after a deposit.',
              'A typical cycle costs a fraction of a cent to a few cents depending on the model.',
            ],
          },
        ],
      },
      {
        id: 'fee-wallet-private-key',
        question: 'Where is the AI fee wallet private key kept?',
        blocks: [
          {
            type: 'p',
            text: 'The key is generated locally on your server, stored encrypted (AES-256) in your own database, and shown to you once in the onboarding screen. Back it up — it cannot be recovered if you lose your database.',
          },
          {
            type: 'note',
            text: 'Keep only fee money in this wallet. It exists to pay for AI calls, not to hold savings.',
          },
        ],
      },
      {
        id: 'hyperliquid-authorization',
        question: 'How does the Hyperliquid authorization work? Is it safe?',
        blocks: [
          {
            type: 'p',
            text: 'NOFX uses Hyperliquid agent wallets, so your main wallet key is never shared. The connect flow has four signed steps:',
          },
          {
            type: 'steps',
            items: [
              'Connect your EVM wallet (Rabby, MetaMask, OKX, Coinbase Wallet).',
              'Approve a freshly generated NOFX agent wallet — valid for 180 days, trading only.',
              'Approve the builder fee (a small per-order fee that funds the platform).',
              'Save the agent key to your NOFX server (stored encrypted).',
            ],
          },
          {
            type: 'p',
            text: 'The agent wallet can place and close orders, nothing else. It cannot withdraw funds, and your collateral stays inside your own Hyperliquid account at all times.',
          },
        ],
      },
      {
        id: 'launch-preflight',
        question: 'What does the launch preflight check?',
        blocks: [
          {
            type: 'p',
            text: 'Before anything is created or changed, the server verifies the full chain with live data:',
          },
          {
            type: 'list',
            items: [
              'AI model is enabled and has a credential.',
              'AI fee wallet key is valid and the Base USDC balance is at least `1 USDC` (queried on-chain).',
              'Hyperliquid account is authorized (agent + builder fee) and reachable.',
              'Trading funds: at least `12 USDC` counting equity in open positions.',
            ],
          },
          {
            type: 'p',
            text: 'Each failing check names the exact fix and deep-links into the guided setup. The same checks are enforced server-side on every start, so the UI cannot be bypassed accidentally.',
          },
        ],
      },
      {
        id: 'relaunch-behavior',
        question: 'What happens if I press Launch again?',
        blocks: [
          {
            type: 'p',
            text: 'Launching is idempotent. If a NOFX Autopilot already exists, the launcher updates it with the current strategy config and restarts it — it never creates a duplicate. A restart can take up to a minute if the bot is mid-cycle; the UI waits for it.',
          },
        ],
      },
      {
        id: 'deposit-not-showing',
        question: 'I deposited USDC but the balance still shows zero.',
        blocks: [
          {
            type: 'list',
            items: [
              'AI fee wallet: make sure you sent USDC on the Base network to the exact address shown. Balances are cached for ~30 seconds, and the setup panel re-checks automatically every few seconds.',
              'Hyperliquid: deposits land in your own Hyperliquid account; the balance step polls the live account state. Use Refresh in the guided panel if in doubt.',
              'If the on-chain RPC is temporarily unreachable, the panel marks the balance as unknown instead of zero — retry in a minute.',
            ],
          },
        ],
      },
    ],
  },

  // ───────────────────────── Trading & execution ─────────────────────────
  {
    id: 'trading',
    title: 'Trading & Execution',
    icon: TrendingUp,
    items: [
      {
        id: 'decision-cycle',
        question: 'How often does the AI make decisions?',
        blocks: [
          {
            type: 'p',
            text: 'The Autopilot runs a scan cycle every 5–15 minutes depending on how you launched it (configurable per trader, minimum 3 minutes). The first cycle starts right after launch; a single cycle usually takes 30–60 seconds because the AI reads the full market context before deciding.',
          },
        ],
      },
      {
        id: 'what-ai-sees',
        question: 'What information does the AI see each cycle?',
        blocks: [
          {
            type: 'list',
            items: [
              'Your account: equity, available margin, open positions with PnL.',
              'The Claw402 ranking board: candidate universe with direction bias.',
              'Signal Lab deep signals and cost/liquidation structure per candidate.',
              'Raw OHLCV candles for timing confirmation.',
              'Its own track record: win rate, profit factor, drawdown, recent trades.',
            ],
          },
          {
            type: 'p',
            text: 'Every cycle is stored as a decision record — the Execution Log on the dashboard shows the reasoning chain, actions, and any blocked orders.',
          },
        ],
      },
      {
        id: 'leverage-and-risk',
        question: 'What leverage and risk controls does it use?',
        blocks: [
          {
            type: 'p',
            text: 'The Autopilot defaults to 10x cross margin. Hard risk controls run outside the AI and cannot be overridden by it:',
          },
          {
            type: 'list',
            items: [
              'Position-count cap from the strategy config — new opens are rejected at the cap.',
              'Leverage limits per asset class (BTC/ETH vs altcoins).',
              'A trade throttle blocks churn, e.g. closing a barely-moved position minutes after opening it.',
              'Safe mode (below) protects the book when the AI itself is failing.',
            ],
          },
        ],
      },
      {
        id: 'safe-mode',
        question: 'What is safe mode?',
        blocks: [
          {
            type: 'p',
            text: 'If the AI fails 3 cycles in a row (provider outage, empty fee wallet, bad responses), the trader enters safe mode: no new positions are opened, existing positions keep their protection, and the loop keeps retrying. It exits safe mode automatically on the next successful AI call.',
          },
          {
            type: 'p',
            text: 'Safe mode is shown as a banner on the dashboard together with the reason, so it never happens silently.',
          },
        ],
      },
      {
        id: 'fee-wallet-empty-mid-run',
        question: 'What happens if the AI fee wallet runs out mid-run?',
        blocks: [
          {
            type: 'p',
            text: 'AI calls start failing with a clear "out of funds" status. The dashboard shows a persistent red banner with the wallet balance, and after three failed cycles the bot enters safe mode. Top up Base USDC to the fee wallet and the trader recovers on its own — no restart needed.',
          },
        ],
      },
      {
        id: 'trading-fees',
        question: 'What fees am I paying?',
        blocks: [
          {
            type: 'list',
            items: [
              'Hyperliquid trading fees on every order, plus the approved builder fee.',
              'AI/data costs paid per call from the fee wallet (cents per cycle).',
            ],
          },
          {
            type: 'p',
            text: 'Fees are the silent killer of high-frequency strategies. The dashboard stats strip shows the full chain — gross realized P/L, minus fees, equals net — so you can see immediately whether fees are eating the edge.',
          },
        ],
      },
      {
        id: 'stop-and-manual',
        question: 'How do I stop the bot or close positions manually?',
        blocks: [
          {
            type: 'list',
            items: [
              'Stop: use the Stop button on the Config page trader list. Stopping halts the decision loop; open positions remain open and are yours to manage.',
              'Manual close: close any position from the dashboard positions panel — manual closes sync back into the position history.',
              'Emergencies: you can always manage positions directly on Hyperliquid; NOFX never locks you out of your own account.',
            ],
          },
        ],
      },
    ],
  },

  // ───────────────────────── Dashboard & metrics ─────────────────────────
  {
    id: 'dashboard',
    title: 'Dashboard & Metrics',
    icon: Monitor,
    items: [
      {
        id: 'metrics-meaning',
        question: 'What do the header metrics mean exactly?',
        blocks: [
          {
            type: 'list',
            items: [
              'Equity — live account value including unrealized PnL.',
              'Total P/L (incl. unrealized) — equity versus your starting balance; moves with open positions.',
              'Realized P/L (closed trades) — net result of finished trades only; this is what win rate, profit factor and sharpe are computed from.',
              'Profit factor — gross wins ÷ gross losses on closed trades; above 1.0 means the closed book is net positive.',
              'Max drawdown — worst peak-to-trough dip of the realized equity curve, measured against your real starting balance.',
            ],
          },
        ],
      },
      {
        id: 'pl-contradiction',
        question: 'Why is Total P/L positive while Realized P/L is negative?',
        blocks: [
          {
            type: 'p',
            text: 'They measure different things. Realized P/L only counts closed trades; Total P/L also includes the unrealized gains of positions still open. A bot can be down on its closed trades while its open book carries enough unrealized profit to put total P/L in the green — and vice versa. Check the gross/fees/net strip to see how much of the realized result is fee drag.',
          },
        ],
      },
      {
        id: 'execution-log',
        question: 'Where can I see why the AI did (or refused) something?',
        blocks: [
          {
            type: 'p',
            text: 'The Execution Log panel lists every cycle with the actions taken, the AI call duration, and any blocked orders with the exact guard that fired (throttle, position cap, risk control). Full reasoning chains are stored with each decision record.',
          },
        ],
      },
      {
        id: 'competition',
        question: 'What is the leaderboard / competition?',
        blocks: [
          {
            type: 'p',
            text: 'Traders with "show in competition" enabled appear on the public leaderboard, ranked by live performance. It is opt-in per trader and can be toggled from the trader list at any time.',
          },
        ],
      },
    ],
  },

  // ───────────────────────── Security ─────────────────────────
  {
    id: 'security',
    title: 'Security',
    icon: Shield,
    items: [
      {
        id: 'key-storage',
        question: 'How are my keys stored?',
        blocks: [
          {
            type: 'list',
            items: [
              'All secrets (agent keys, fee wallet key, exchange API keys) are AES-256 encrypted at rest in your own database.',
              'Optional RSA transport encryption protects secrets in flight between browser and server.',
              'NOFX is self-hosted: nothing is sent to any third-party server. The code is open source and auditable.',
            ],
          },
        ],
      },
      {
        id: 'can-nofx-steal-funds',
        question: 'Can NOFX withdraw or steal my funds?',
        blocks: [
          {
            type: 'p',
            text: 'No. On Hyperliquid, NOFX only ever holds an agent wallet, which by protocol design can trade but cannot withdraw. Your collateral stays in your own account under your main wallet’s control.',
          },
          {
            type: 'note',
            text: 'If you connect a CEX instead, create its API key with trading permission only — disable withdrawals and set an IP whitelist.',
          },
        ],
      },
      {
        id: 'registration-model',
        question: 'Why can’t anyone else register on my instance?',
        blocks: [
          {
            type: 'p',
            text: 'By design, an instance is single-operator: the first account registered becomes the operator and registration closes ("System already initialized"). This prevents strangers from creating accounts on an exposed deployment. Run one instance per operator.',
          },
        ],
      },
    ],
  },

  // ───────────────────────── Self-hosting ─────────────────────────
  {
    id: 'self-hosting',
    title: 'Self-Hosting & Troubleshooting',
    icon: Wrench,
    items: [
      {
        id: 'how-to-install',
        question: 'How do I install NOFX?',
        blocks: [
          {
            type: 'p',
            text: 'One line on Linux/macOS (installs and starts everything via Docker):',
          },
          {
            type: 'list',
            items: [
              'Script: `curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash`',
              'Docker: download `docker-compose.prod.yml` and run `docker compose -f docker-compose.prod.yml up -d`',
              'Windows: install Docker Desktop, then use the Docker route above.',
              'From source: Go 1.21+, Node 18+, TA-Lib (`brew install ta-lib` / `apt-get install libta-lib0-dev`), then `go run .` and `npm --prefix web run dev`.',
            ],
          },
          {
            type: 'p',
            text: 'Then open `http://127.0.0.1:3000` — the web UI is on port 3000, the API on 8080.',
          },
        ],
      },
      {
        id: 'how-to-update',
        question: 'How do I update?',
        blocks: [
          {
            type: 'p',
            text: 'Re-run the install script, or with Docker: `docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d`. Your database and keys live in the mounted `data/` directory and survive updates. Running traders are restarted automatically after the backend comes back.',
          },
        ],
      },
      {
        id: 'launch-blocked',
        question: 'Launch is blocked by a failing check — now what?',
        blocks: [
          {
            type: 'p',
            text: 'Read the message: every preflight failure names its fix and routes you to the right setup step — fund the AI wallet, finish the Hyperliquid authorization, or deposit trading USDC. Balances are re-checked live, so once you fix the item the launch goes through.',
          },
        ],
      },
      {
        id: 'exchange-unreachable',
        question: 'The exchange account shows "invalid credentials" or "unavailable".',
        blocks: [
          {
            type: 'list',
            items: [
              'Invalid credentials: the agent authorization expired (180 days) or the saved key is stale — reconnect the Hyperliquid wallet; the flow offers a one-click renewal.',
              'Unavailable: the exchange API did not respond; the account state is cached for 30 seconds, so wait and refresh.',
              'CEX keys: verify trading permission, IP whitelist, and that futures/perp access is enabled.',
            ],
          },
        ],
      },
      {
        id: 'where-are-logs',
        question: 'Where are the logs?',
        blocks: [
          {
            type: 'list',
            items: [
              'Backend: `docker logs nofx-trading` (or the terminal running `go run .`).',
              'Per-cycle AI reasoning and errors: the dashboard Execution Log.',
              'Frontend build/runtime issues: browser devtools console.',
            ],
          },
        ],
      },
      {
        id: 'port-conflicts',
        question: 'Port 3000 or 8080 is already in use.',
        blocks: [
          {
            type: 'p',
            text: 'Stop the conflicting service or remap the published ports in your compose file (e.g. `"3100:80"` for the frontend, `"8180:8080"` for the API), then restart the containers.',
          },
        ],
      },
    ],
  },

  // ───────────────────────── Contributing ─────────────────────────
  {
    id: 'contributing',
    title: 'Contributing',
    icon: GitBranch,
    items: [
      {
        id: 'how-to-contribute',
        question: 'How do I contribute code?',
        blocks: [
          {
            type: 'links',
            links: [
              {
                label: 'Roadmap',
                href: 'https://github.com/orgs/NoFxAiOS/projects/3',
              },
              {
                label: 'Task Dashboard',
                href: 'https://github.com/orgs/NoFxAiOS/projects/5',
              },
              {
                label: 'CONTRIBUTING.md',
                href: 'https://github.com/NoFxAiOS/nofx/blob/dev/CONTRIBUTING.md',
              },
            ],
          },
          {
            type: 'steps',
            items: [
              'Pick a task from the boards above (filter by good first issue / help wanted) and comment "assign me".',
              'Fork the repo and branch from `dev`: `git checkout -b feat/your-topic`.',
              'Follow Conventional Commits; run `npm --prefix web run lint && npm --prefix web run build` before pushing.',
              'Open a PR against `NoFxAiOS/nofx:dev`, reference the issue (`Closes #123`), and attach screenshots for UI changes.',
            ],
          },
        ],
      },
      {
        id: 'bounty-program',
        question: 'Is there a bounty program?',
        blocks: [
          {
            type: 'p',
            text: 'Yes — selected issues carry cash bounties, plus badges, priority review, and beta access for regular contributors.',
          },
          {
            type: 'links',
            links: [
              {
                label: 'Issues with bounty label',
                href: 'https://github.com/NoFxAiOS/nofx/labels/bounty',
              },
              {
                label: 'Bounty claim template',
                href: 'https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md',
              },
            ],
          },
        ],
      },
      {
        id: 'report-bugs',
        question: 'How do I report a bug?',
        blocks: [
          {
            type: 'p',
            text: 'Open a GitHub issue with the template: what you did, what happened, backend logs (`docker logs nofx-trading`), and screenshots. For suspected security issues, please follow the responsible-disclosure notes in SECURITY.md instead of a public issue.',
          },
          {
            type: 'links',
            links: [
              {
                label: 'New issue',
                href: 'https://github.com/NoFxAiOS/nofx/issues/new/choose',
              },
              {
                label: 'SECURITY.md',
                href: 'https://github.com/NoFxAiOS/nofx/blob/dev/SECURITY.md',
              },
            ],
          },
        ],
      },
    ],
  },
]
