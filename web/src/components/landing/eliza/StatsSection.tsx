import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { Github } from 'lucide-react'
import { OFFICIAL_LINKS } from '../../../constants/branding'

interface StatsData {
  stars: string
  contributors: string
  forks: string
  watchers: string
}

export default function StatsSection() {
  const [stats, setStats] = useState<StatsData>({
    stars: '-',
    contributors: '-',
    forks: '-',
    watchers: '-'
  })

  useEffect(() => {
    // Fetch repo data
    fetch('https://api.github.com/repos/NoFxAiOS/nofx')
      .then(res => res.json())
      .then(data => {
        if (data.stargazers_count !== undefined) {
          setStats(prev => ({
            ...prev,
            stars: formatNumber(data.stargazers_count),
            forks: formatNumber(data.forks_count),
            watchers: formatNumber(data.subscribers_count || data.watchers_count)
          }))
        }
      })
      .catch(() => {})

    // Fetch contributors count
    fetch('https://api.github.com/repos/NoFxAiOS/nofx/contributors?per_page=1')
      .then(res => {
        const linkHeader = res.headers.get('Link')
        if (linkHeader) {
          const match = linkHeader.match(/page=(\d+)>; rel="last"/)
          if (match) {
            setStats(prev => ({
              ...prev,
              contributors: formatNumber(parseInt(match[1]))
            }))
          }
        } else {
          return res.json().then(data => {
            setStats(prev => ({
              ...prev,
              contributors: formatNumber(Array.isArray(data) ? data.length : 1)
            }))
          })
        }
      })
      .catch(() => {})
  }, [])

  const formatNumber = (num: number): string => {
    if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K'
    }
    return num.toLocaleString()
  }

  const statsItems = [
    { label: 'STARS', value: stats.stars },
    { label: 'CONTRIBUTORS', value: stats.contributors },
    { label: 'FORKS', value: stats.forks },
    { label: 'WATCHERS', value: stats.watchers },
  ]

  return (
    <section
      className="w-full min-h-screen bg-nofx-gold flex items-center"
      style={{ scrollSnapAlign: 'start' }}
    >
      <div className="max-w-7xl mx-auto px-6 py-20 w-full">

        {/* Header */}
        <motion.div
          className="flex flex-col md:flex-row md:items-center md:justify-between gap-6 mb-20"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          viewport={{ once: true, amount: 0.3 }}
        >
          <div>
            <p className="text-black/60 text-sm font-bold uppercase tracking-wider">
              By Developers, for Developers
            </p>
          </div>
          <a
            href={OFFICIAL_LINKS.github}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 text-black/70 hover:text-black transition-colors"
          >
            <Github className="w-5 h-5" />
            <span className="text-sm font-medium">GitHub Stats</span>
          </a>
        </motion.div>

        {/* Stats Grid - Fixed layout */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-x-8 gap-y-12">
          {statsItems.map((item, index) => (
            <motion.div
              key={index}
              className="text-left"
              initial={{ opacity: 0, y: 50, scale: 0.9 }}
              whileInView={{ opacity: 1, y: 0, scale: 1 }}
              transition={{ duration: 0.6, delay: index * 0.15, ease: 'easeOut' }}
              viewport={{ once: true, amount: 0.3 }}
            >
              <div className="text-5xl sm:text-6xl md:text-7xl font-black text-black mb-3 leading-none tracking-tight">
                {item.value}
              </div>
              <div className="text-black/50 text-xs uppercase tracking-wider font-bold">
                {item.label}
              </div>
            </motion.div>
          ))}
        </div>

      </div>
    </section>
  )
}
