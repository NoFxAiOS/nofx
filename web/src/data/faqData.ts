import {
  BookOpen,
  Settings,
  TrendingUp,
  Wrench,
  Bot,
  Shield,
  Monitor,
  Zap,
  GitBranch,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface FAQItem {
  id: string
  questionKey: string
  answerKey: string
}

export interface FAQCategory {
  id: string
  titleKey: string
  icon: LucideIcon
  items: FAQItem[]
}

/**
 * FAQ Data Configuration
 * Comprehensive FAQ covering all aspects of NOFX
 *
 * Categories:
 * 1. Getting Started - Basic concepts and overview
 * 2. Installation - Setup methods and requirements
 * 3. Configuration - AI models, exchanges, strategies
 * 4. Trading - How trading works, common issues
 * 5. Technical Issues - Troubleshooting common problems
 * 6. Security - API keys, encryption, best practices
 * 7. Features - Strategy Studio, Backtest, Debate Arena
 * 8. Contributing - How to contribute to the project
 */
export const faqCategories: FAQCategory[] = [
  // ===== 1. GETTING STARTED =====
  {
    id: 'getting-started',
    titleKey: 'faqCategoryGettingStarted',
    icon: BookOpen,
    items: [
      {
        id: 'what-is-nofx',
        questionKey: 'faqWhatIsNOFX',
        answerKey: 'faqWhatIsNOFXAnswer',
      },
      {
        id: 'how-does-it-work',
        questionKey: 'faqHowDoesItWork',
        answerKey: 'faqHowDoesItWorkAnswer',
      },
      {
        id: 'is-it-profitable',
        questionKey: 'faqIsProfitable',
        answerKey: 'faqIsProfitableAnswer',
      },
      {
        id: 'supported-exchanges',
        questionKey: 'faqSupportedExchanges',
        answerKey: 'faqSupportedExchangesAnswer',
      },
      {
        id: 'supported-ai-models',
        questionKey: 'faqSupportedAIModels',
        answerKey: 'faqSupportedAIModelsAnswer',
      },
      {
        id: 'system-requirements',
        questionKey: 'faqSystemRequirements',
        answerKey: 'faqSystemRequirementsAnswer',
      },
    ],
  },

  // ===== 2. INSTALLATION =====
  {
    id: 'installation',
    titleKey: 'faqCategoryInstallation',
    icon: Settings,
    items: [
      {
        id: 'how-to-install',
        questionKey: 'faqHowToInstall',
        answerKey: 'faqHowToInstallAnswer',
      },
      {
        id: 'windows-installation',
        questionKey: 'faqWindowsInstallation',
        answerKey: 'faqWindowsInstallationAnswer',
      },
      {
        id: 'docker-deployment',
        questionKey: 'faqDockerDeployment',
        answerKey: 'faqDockerDeploymentAnswer',
      },
      {
        id: 'manual-installation',
        questionKey: 'faqManualInstallation',
        answerKey: 'faqManualInstallationAnswer',
      },
      {
        id: 'server-deployment',
        questionKey: 'faqServerDeployment',
        answerKey: 'faqServerDeploymentAnswer',
      },
      {
        id: 'update-nofx',
        questionKey: 'faqUpdateNOFX',
        answerKey: 'faqUpdateNOFXAnswer',
      },
    ],
  },

  // ===== 3. CONFIGURATION =====
  {
    id: 'configuration',
    titleKey: 'faqCategoryConfiguration',
    icon: Zap,
    items: [
      {
        id: 'configure-ai-models',
        questionKey: 'faqConfigureAIModels',
        answerKey: 'faqConfigureAIModelsAnswer',
      },
      {
        id: 'configure-exchanges',
        questionKey: 'faqConfigureExchanges',
        answerKey: 'faqConfigureExchangesAnswer',
      },
      {
        id: 'binance-api-setup',
        questionKey: 'faqBinanceAPISetup',
        answerKey: 'faqBinanceAPISetupAnswer',
      },
      {
        id: 'hyperliquid-setup',
        questionKey: 'faqHyperliquidSetup',
        answerKey: 'faqHyperliquidSetupAnswer',
      },
      {
        id: 'create-strategy',
        questionKey: 'faqCreateStrategy',
        answerKey: 'faqCreateStrategyAnswer',
      },
      {
        id: 'create-trader',
        questionKey: 'faqCreateTrader',
        answerKey: 'faqCreateTraderAnswer',
      },
    ],
  },

  // ===== 4. TRADING =====
  {
    id: 'trading',
    titleKey: 'faqCategoryTrading',
    icon: TrendingUp,
    items: [
      {
        id: 'how-ai-decides',
        questionKey: 'faqHowAIDecides',
        answerKey: 'faqHowAIDecidesAnswer',
      },
      {
        id: 'decision-frequency',
        questionKey: 'faqDecisionFrequency',
        answerKey: 'faqDecisionFrequencyAnswer',
      },
      {
        id: 'no-trades-executing',
        questionKey: 'faqNoTradesExecuting',
        answerKey: 'faqNoTradesExecutingAnswer',
      },
      {
        id: 'only-short-positions',
        questionKey: 'faqOnlyShortPositions',
        answerKey: 'faqOnlyShortPositionsAnswer',
      },
      {
        id: 'leverage-settings',
        questionKey: 'faqLeverageSettings',
        answerKey: 'faqLeverageSettingsAnswer',
      },
      {
        id: 'stop-loss-take-profit',
        questionKey: 'faqStopLossTakeProfit',
        answerKey: 'faqStopLossTakeProfitAnswer',
      },
      {
        id: 'multiple-traders',
        questionKey: 'faqMultipleTraders',
        answerKey: 'faqMultipleTradersAnswer',
      },
      {
        id: 'ai-costs',
        questionKey: 'faqAICosts',
        answerKey: 'faqAICostsAnswer',
      },
    ],
  },

  // ===== 5. TECHNICAL ISSUES =====
  {
    id: 'technical-issues',
    titleKey: 'faqCategoryTechnicalIssues',
    icon: Wrench,
    items: [
      {
        id: 'port-in-use',
        questionKey: 'faqPortInUse',
        answerKey: 'faqPortInUseAnswer',
      },
      {
        id: 'frontend-not-loading',
        questionKey: 'faqFrontendNotLoading',
        answerKey: 'faqFrontendNotLoadingAnswer',
      },
      {
        id: 'database-locked',
        questionKey: 'faqDatabaseLocked',
        answerKey: 'faqDatabaseLockedAnswer',
      },
      {
        id: 'talib-not-found',
        questionKey: 'faqTALibNotFound',
        answerKey: 'faqTALibNotFoundAnswer',
      },
      {
        id: 'ai-api-timeout',
        questionKey: 'faqAIAPITimeout',
        answerKey: 'faqAIAPITimeoutAnswer',
      },
      {
        id: 'binance-position-mode',
        questionKey: 'faqBinancePositionMode',
        answerKey: 'faqBinancePositionModeAnswer',
      },
      {
        id: 'balance-shows-zero',
        questionKey: 'faqBalanceShowsZero',
        answerKey: 'faqBalanceShowsZeroAnswer',
      },
      {
        id: 'docker-pull-failed',
        questionKey: 'faqDockerPullFailed',
        answerKey: 'faqDockerPullFailedAnswer',
      },
    ],
  },

  // ===== 6. SECURITY =====
  {
    id: 'security',
    titleKey: 'faqCategorySecurity',
    icon: Shield,
    items: [
      {
        id: 'api-key-storage',
        questionKey: 'faqAPIKeyStorage',
        answerKey: 'faqAPIKeyStorageAnswer',
      },
      {
        id: 'encryption-details',
        questionKey: 'faqEncryptionDetails',
        answerKey: 'faqEncryptionDetailsAnswer',
      },
      {
        id: 'security-best-practices',
        questionKey: 'faqSecurityBestPractices',
        answerKey: 'faqSecurityBestPracticesAnswer',
      },
      {
        id: 'can-nofx-steal-funds',
        questionKey: 'faqCanNOFXStealFunds',
        answerKey: 'faqCanNOFXStealFundsAnswer',
      },
    ],
  },

  // ===== 7. FEATURES =====
  {
    id: 'features',
    titleKey: 'faqCategoryFeatures',
    icon: Monitor,
    items: [
      {
        id: 'strategy-studio',
        questionKey: 'faqStrategyStudio',
        answerKey: 'faqStrategyStudioAnswer',
      },
      {
        id: 'backtest-lab',
        questionKey: 'faqBacktestLab',
        answerKey: 'faqBacktestLabAnswer',
      },
      {
        id: 'debate-arena',
        questionKey: 'faqDebateArena',
        answerKey: 'faqDebateArenaAnswer',
      },
      {
        id: 'competition-mode',
        questionKey: 'faqCompetitionMode',
        answerKey: 'faqCompetitionModeAnswer',
      },
      {
        id: 'chain-of-thought',
        questionKey: 'faqChainOfThought',
        answerKey: 'faqChainOfThoughtAnswer',
      },
    ],
  },

  // ===== 8. AI MODELS =====
  {
    id: 'ai-models',
    titleKey: 'faqCategoryAIModels',
    icon: Bot,
    items: [
      {
        id: 'which-ai-model-best',
        questionKey: 'faqWhichAIModelBest',
        answerKey: 'faqWhichAIModelBestAnswer',
      },
      {
        id: 'custom-ai-api',
        questionKey: 'faqCustomAIAPI',
        answerKey: 'faqCustomAIAPIAnswer',
      },
      {
        id: 'ai-hallucinations',
        questionKey: 'faqAIHallucinations',
        answerKey: 'faqAIHallucinationsAnswer',
      },
      {
        id: 'compare-ai-models',
        questionKey: 'faqCompareAIModels',
        answerKey: 'faqCompareAIModelsAnswer',
      },
    ],
  },

  // ===== 9. CONTRIBUTING =====
  {
    id: 'contributing',
    titleKey: 'faqCategoryContributing',
    icon: GitBranch,
    items: [
      {
        id: 'how-to-contribute',
        questionKey: 'faqHowToContribute',
        answerKey: 'faqHowToContributeAnswer',
      },
      {
        id: 'pr-guidelines',
        questionKey: 'faqPRGuidelines',
        answerKey: 'faqPRGuidelinesAnswer',
      },
      {
        id: 'bounty-program',
        questionKey: 'faqBountyProgram',
        answerKey: 'faqBountyProgramAnswer',
      },
      {
        id: 'report-bugs',
        questionKey: 'faqReportBugs',
        answerKey: 'faqReportBugsAnswer',
      },
    ],
  },
]
