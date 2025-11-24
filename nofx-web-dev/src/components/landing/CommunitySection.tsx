import { motion } from 'framer-motion'
import AnimatedSection from './AnimatedSection'

interface CardProps {
  quote: string
  authorName: string
  handle: string
  avatarUrl: string
  tweetUrl: string
  delay: number
}

function TestimonialCard({ quote, authorName, delay }: CardProps) {
  return (
    <motion.div
      className="p-6 rounded-xl"
      style={{
        background: 'var(--panel-bg)',
        border: '1px solid var(--panel-border)',
      }}
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay }}
      whileHover={{ scale: 1.05 }}
    >
      <p className="text-lg mb-4" style={{ color: 'var(--text-primary)' }}>
        "{quote}"
      </p>
      <div className="flex items-center gap-2">
        <div
          className="w-8 h-8 rounded-full"
          style={{ background: 'var(--brand-red)' }}
        />
        <span
          className="text-sm font-semibold"
          style={{ color: 'var(--text-secondary)' }}
        >
          {authorName}
        </span>
      </div>
    </motion.div>
  )
}

export default function CommunitySection() {
  const staggerContainer = {
    animate: { transition: { staggerChildren: 0.1 } },
  }

  // 推特内容整合（保持原三列布局，超出自动换行）
  const items: CardProps[] = [
    {
      quote:
        '前不久非常火的 AI 量化交易系统 NOF1，在社区有人将其开源，这就是智慧云数项目。基于 DeepSeek、Qwen 等大语言模型，打造的通用架构 AI 交易操作系统，完成了从决策、到交易、再到复盘的闭环。',
      authorName: 'Michael Williams',
      handle: '@MichaelWil93725',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1767615411594694659/Mj8Fdt6o_400x400.jpg',
      tweetUrl:
        'https://twitter.com/MichaelWil93725/status/1984980920395604008',
      delay: 0,
    },
    {
      quote:
        '跑了一晚上 @nofx_ai 开源的 AI 自动交易，太有意思了，就看 AI 在那一会开空一会开多，一顿操作，虽然看不懂为什么，但是一晚上帮我赚了 6% 收益',
      authorName: 'DIŸgöd',
      handle: '@DIYgod',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1628393369029181440/r23HDDJk_400x400.jpg',
      tweetUrl: 'https://twitter.com/DIYgod/status/1984442354515017923',
      delay: 0.1,
    },
    {
      quote:
        'Open-source 智慧云数 revives the legendary Alpha Arena, an AI-powered crypto futures battleground. Built on DeepSeek/Qwen AI, it trades live on Binance, Hyperliquid, and Aster DEX, featuring multi-AI battles and self-learning bots',
      authorName: 'Kai',
      handle: '@hqmank',
      avatarUrl:
        'https://pbs.twimg.com/profile_images/1905441261911506945/4YhLIqUm_400x400.jpg',
      tweetUrl: 'https://twitter.com/hqmank/status/1984227431994290340',
      delay: 0.15,
    },
  ]

  return (
    <AnimatedSection>
      <div className="max-w-7xl mx-auto">
        <motion.div
          className="grid md:grid-cols-3 gap-6"
          variants={staggerContainer}
          initial="initial"
          whileInView="animate"
          viewport={{ once: true }}
        >
          {items.map((item, idx) => (
            <TestimonialCard key={idx} {...item} />
          ))}
        </motion.div>
      </div>
    </AnimatedSection>
  )
}
