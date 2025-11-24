import { BookOpen, TrendingUp, Bot } from 'lucide-react'
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
]
