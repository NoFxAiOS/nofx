import {
  BookOpen,
  Settings,
  TrendingUp,
  Wrench,
  Bot,
  Database,
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
 * FAQ 数据配置
 * - titleKey: 分类标题的翻译键
 * - questionKey: 问题的翻译键
 * - answerKey: 答案的翻译键
 *
 * 所有文本内容都通过翻译键从 i18n/translations.ts 获取
 */
export const faqCategories: FAQCategory[] = [
  {
    id: 'basics',
    titleKey: 'faqCategoryBasics',
    icon: BookOpen,
    items: [
      {
        id: 'what-is-nofx',
        questionKey: 'faqWhatIsNOFX',
        answerKey: 'faqWhatIsNOFXAnswer',
      },
      {
        id: 'supported-exchanges',
        questionKey: 'faqSupportedExchanges',
        answerKey: 'faqSupportedExchangesAnswer',
      },
      {
        id: 'is-profitable',
        questionKey: 'faqIsProfitable',
        answerKey: 'faqIsProfitableAnswer',
      },
      {
        id: 'multiple-traders',
        questionKey: 'faqMultipleTraders',
        answerKey: 'faqMultipleTradersAnswer',
      },
    ],
  },
  {
    id: 'contributing',
    titleKey: 'faqCategoryContributing',
    icon: GitBranch,
    items: [
      {
        id: 'github-projects-tasks',
        questionKey: 'faqGithubProjectsTasks',
        answerKey: 'faqGithubProjectsTasksAnswer',
      },
      {
        id: 'contribute-pr-guidelines',
        questionKey: 'faqContributePR',
        answerKey: 'faqContributePRAnswer',
      },
    ],
  },
  {
    id: 'setup',
    titleKey: 'faqCategorySetup',
    icon: Settings,
    items: [
      {
        id: 'system-requirements',
        questionKey: 'faqSystemRequirements',
        answerKey: 'faqSystemRequirementsAnswer',
      },
      {
        id: 'need-coding',
        questionKey: 'faqNeedCoding',
        answerKey: 'faqNeedCodingAnswer',
      },
      {
        id: 'get-api-keys',
        questionKey: 'faqGetApiKeys',
        answerKey: 'faqGetApiKeysAnswer',
      },
      {
        id: 'use-subaccount',
        questionKey: 'faqUseSubaccount',
        answerKey: 'faqUseSubaccountAnswer',
      },
      {
        id: 'docker-deployment',
        questionKey: 'faqDockerDeployment',
        answerKey: 'faqDockerDeploymentAnswer',
      },
      {
        id: 'balance-shows-zero',
        questionKey: 'faqBalanceZero',
        answerKey: 'faqBalanceZeroAnswer',
      },
      {
        id: 'testnet-issues',
        questionKey: 'faqTestnet',
        answerKey: 'faqTestnetAnswer',
      },
    ],
  },
  {
    id: 'trading',
    titleKey: 'faqCategoryTrading',
    icon: TrendingUp,
    items: [
      {
        id: 'no-trades',
        questionKey: 'faqNoTrades',
        answerKey: 'faqNoTradesAnswer',
      },
      {
        id: 'decision-frequency',
        questionKey: 'faqDecisionFrequency',
        answerKey: 'faqDecisionFrequencyAnswer',
      },
      {
        id: 'custom-strategy',
        questionKey: 'faqCustomStrategy',
        answerKey: 'faqCustomStrategyAnswer',
      },
      {
        id: 'max-positions',
        questionKey: 'faqMaxPositions',
        answerKey: 'faqMaxPositionsAnswer',
      },
      {
        id: 'margin-insufficient',
        questionKey: 'faqMarginInsufficient',
        answerKey: 'faqMarginInsufficientAnswer',
      },
      {
        id: 'high-fees',
        questionKey: 'faqHighFees',
        answerKey: 'faqHighFeesAnswer',
      },
      {
        id: 'no-take-profit',
        questionKey: 'faqNoTakeProfit',
        answerKey: 'faqNoTakeProfitAnswer',
      },
    ],
  },
  {
    id: 'technical',
    titleKey: 'faqCategoryTechnical',
    icon: Wrench,
    items: [
      {
        id: 'binance-api-failed',
        questionKey: 'faqBinanceApiFailed',
        answerKey: 'faqBinanceApiFailedAnswer',
      },
      {
        id: 'binance-position-mode',
        questionKey: 'faqBinancePositionMode',
        answerKey: 'faqBinancePositionModeAnswer',
      },
      {
        id: 'port-in-use',
        questionKey: 'faqPortInUse',
        answerKey: 'faqPortInUseAnswer',
      },
      {
        id: 'frontend-loading',
        questionKey: 'faqFrontendLoading',
        answerKey: 'faqFrontendLoadingAnswer',
      },
      {
        id: 'database-locked',
        questionKey: 'faqDatabaseLocked',
        answerKey: 'faqDatabaseLockedAnswer',
      },
      {
        id: 'ai-learning-failed',
        questionKey: 'faqAiLearningFailed',
        answerKey: 'faqAiLearningFailedAnswer',
      },
      {
        id: 'config-not-effective',
        questionKey: 'faqConfigNotEffective',
        answerKey: 'faqConfigNotEffectiveAnswer',
      },
    ],
  },
  {
    id: 'ai',
    titleKey: 'faqCategoryAI',
    icon: Bot,
    items: [
      {
        id: 'which-models',
        questionKey: 'faqWhichModels',
        answerKey: 'faqWhichModelsAnswer',
      },
      {
        id: 'api-costs',
        questionKey: 'faqApiCosts',
        answerKey: 'faqApiCostsAnswer',
      },
      {
        id: 'multiple-models',
        questionKey: 'faqMultipleModels',
        answerKey: 'faqMultipleModelsAnswer',
      },
      {
        id: 'ai-learning',
        questionKey: 'faqAiLearning',
        answerKey: 'faqAiLearningAnswer',
      },
      {
        id: 'only-short-positions',
        questionKey: 'faqOnlyShort',
        answerKey: 'faqOnlyShortAnswer',
      },
      {
        id: 'model-selection',
        questionKey: 'faqModelSelection',
        answerKey: 'faqModelSelectionAnswer',
      },
    ],
  },
  {
    id: 'data',
    titleKey: 'faqCategoryData',
    icon: Database,
    items: [
      {
        id: 'data-storage',
        questionKey: 'faqDataStorage',
        answerKey: 'faqDataStorageAnswer',
      },
      {
        id: 'api-key-security',
        questionKey: 'faqApiKeySecurity',
        answerKey: 'faqApiKeySecurityAnswer',
      },
      {
        id: 'export-history',
        questionKey: 'faqExportHistory',
        answerKey: 'faqExportHistoryAnswer',
      },
      {
        id: 'get-help',
        questionKey: 'faqGetHelp',
        answerKey: 'faqGetHelpAnswer',
      },
    ],
  },
]
