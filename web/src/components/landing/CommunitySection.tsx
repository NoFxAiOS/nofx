import { motion } from 'framer-motion'
import { MessageCircle, Heart, Repeat2, ExternalLink } from 'lucide-react'
import { Language } from '../../i18n/translations'

interface TweetProps {
  quote: string
  authorName: string
  handle: string
  avatarUrl: string
  tweetUrl: string
  delay: number
}

function TweetCard({ quote, authorName, handle, avatarUrl, tweetUrl, delay }: TweetProps) {
  return (
    <motion.a
      href={tweetUrl}
      target="_blank"
      rel="noopener noreferrer"
      className="block p-5 rounded-2xl transition-all duration-300 group"
      style={{
        background: '#12161C',
        border: '1px solid rgba(255, 255, 255, 0.06)',
      }}
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay }}
      whileHover={{
        y: -4,
        borderColor: 'rgba(240, 185, 11, 0.3)',
      }}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          <img
            src={avatarUrl}
            alt={authorName}
            className="w-10 h-10 rounded-full object-cover"
            style={{ border: '2px solid rgba(255, 255, 255, 0.1)' }}
          />
          <div>
            <div className="font-semibold text-sm" style={{ color: '#EAECEF' }}>
              {authorName}
            </div>
            <div className="text-xs" style={{ color: '#5E6673' }}>
              {handle}
            </div>
          </div>
        </div>
        {/* X Logo */}
        <div
          className="w-6 h-6 flex items-center justify-center opacity-50 group-hover:opacity-100 transition-opacity"
          style={{ color: '#EAECEF' }}
        >
          <svg viewBox="0 0 24 24" className="w-4 h-4" fill="currentColor">
            <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
          </svg>
        </div>
      </div>

      {/* Content */}
      <p
        className="text-sm leading-relaxed mb-4 line-clamp-4"
        style={{ color: '#B7BDC6' }}
      >
        {quote}
      </p>

      {/* Footer */}
      <div className="flex items-center gap-6 pt-3" style={{ borderTop: '1px solid rgba(255, 255, 255, 0.05)' }}>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#5E6673' }}>
          <MessageCircle className="w-3.5 h-3.5" />
          <span>Reply</span>
        </div>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#5E6673' }}>
          <Repeat2 className="w-3.5 h-3.5" />
          <span>Repost</span>
        </div>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#5E6673' }}>
          <Heart className="w-3.5 h-3.5" />
          <span>Like</span>
        </div>
        <div className="ml-auto opacity-0 group-hover:opacity-100 transition-opacity">
          <ExternalLink className="w-3.5 h-3.5" style={{ color: '#F0B90B' }} />
        </div>
      </div>
    </motion.a>
  )
}

interface CommunitySectionProps {
  language?: Language
}

export default function CommunitySection({ language }: CommunitySectionProps) {
  const tweets: TweetProps[] = []

  return (
    <section className="py-24 relative" style={{ background: '#0B0E11' }}>
      {/* Background Decoration */}
      <div
        className="absolute right-0 top-1/2 -translate-y-1/2 w-96 h-96 rounded-full blur-3xl opacity-20"
        style={{ background: 'radial-gradient(circle, rgba(29, 161, 242, 0.1) 0%, transparent 70%)' }}
      />

      <div className="max-w-6xl mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          className="text-center mb-12"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="text-4xl lg:text-5xl font-bold mb-4" style={{ color: '#EAECEF' }}>
            {language === 'zh' ? '社区声音' : 'Community Voices'}
          </h2>
          <p className="text-lg" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '看看大家怎么说' : 'See what others are saying'}
          </p>
        </motion.div>

        {/* Tweet Grid */}
        <div className="grid md:grid-cols-3 gap-5">
          {tweets.map((tweet, idx) => (
            <TweetCard key={idx} {...tweet} />
          ))}
        </div>

        {/* CTA */}
        <motion.div
          className="text-center mt-12"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <a
            href="https://twitter.com/nofx_official"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-6 py-3 rounded-xl font-medium transition-all hover:scale-105"
            style={{
              background: 'rgba(29, 161, 242, 0.1)',
              color: '#1DA1F2',
              border: '1px solid rgba(29, 161, 242, 0.3)',
            }}
          >
            <svg viewBox="0 0 24 24" className="w-5 h-5" fill="currentColor">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            {language === 'zh' ? '关注我们的 X' : 'Follow us on X'}
          </a>
        </motion.div>
      </div>
    </section>
  )
}
