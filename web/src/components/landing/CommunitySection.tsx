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
        background: '#F7F4EC',
        border: '1px solid rgba(26, 24, 19, 0.14)',
      }}
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ delay }}
      whileHover={{
        y: -4,
        borderColor: 'rgba(224, 72, 59, 0.3)',
      }}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          <img
            src={avatarUrl}
            alt={authorName}
            className="w-10 h-10 rounded-full object-cover"
            style={{ border: '2px solid rgba(26, 24, 19, 0.14)' }}
          />
          <div>
            <div className="font-semibold text-sm" style={{ color: '#1A1813' }}>
              {authorName}
            </div>
            <div className="text-xs" style={{ color: '#8A8478' }}>
              {handle}
            </div>
          </div>
        </div>
        {/* X Logo */}
        <div
          className="w-6 h-6 flex items-center justify-center opacity-50 group-hover:opacity-100 transition-opacity"
          style={{ color: '#1A1813' }}
        >
          <svg viewBox="0 0 24 24" className="w-4 h-4" fill="currentColor">
            <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
          </svg>
        </div>
      </div>

      {/* Content */}
      <p
        className="text-sm leading-relaxed mb-4 line-clamp-4"
        style={{ color: '#1A1813' }}
      >
        {quote}
      </p>

      {/* Footer */}
      <div className="flex items-center gap-6 pt-3" style={{ borderTop: '1px solid rgba(26, 24, 19, 0.14)' }}>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#8A8478' }}>
          <MessageCircle className="w-3.5 h-3.5" />
          <span>Reply</span>
        </div>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#8A8478' }}>
          <Repeat2 className="w-3.5 h-3.5" />
          <span>Repost</span>
        </div>
        <div className="flex items-center gap-1.5 text-xs" style={{ color: '#8A8478' }}>
          <Heart className="w-3.5 h-3.5" />
          <span>Like</span>
        </div>
        <div className="ml-auto opacity-0 group-hover:opacity-100 transition-opacity">
          <ExternalLink className="w-3.5 h-3.5" style={{ color: '#E0483B' }} />
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
    <section className="py-24 relative" style={{ background: '#F1ECE2' }}>
      {/* Background Decoration */}
      <div
        className="absolute right-0 top-1/2 -translate-y-1/2 w-96 h-96 rounded-full blur-3xl opacity-20"
        style={{ background: 'radial-gradient(circle, rgba(224, 72, 59, 0.08) 0%, transparent 70%)' }}
      />

      <div className="max-w-6xl mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          className="text-center mb-12"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="text-4xl lg:text-5xl font-bold mb-4" style={{ color: '#1A1813' }}>
            {language === 'zh' ? 'Community Voices' : 'Community Voices'}
          </h2>
          <p className="text-lg" style={{ color: '#8A8478' }}>
            {language === 'zh' ? 'See what others are saying' : 'See what others are saying'}
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
            href="https://x.com/vergex_ai"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-6 py-3 rounded-xl font-medium transition-all hover:scale-105"
            style={{
              background: 'rgba(224, 72, 59, 0.1)',
              color: '#E0483B',
              border: '1px solid rgba(224, 72, 59, 0.3)',
            }}
          >
            <svg viewBox="0 0 24 24" className="w-5 h-5" fill="currentColor">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            {language === 'zh' ? 'Follow us on X' : 'Follow us on X'}
          </a>
        </motion.div>
      </div>
    </section>
  )
}
