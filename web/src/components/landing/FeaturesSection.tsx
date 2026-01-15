import { motion } from 'framer-motion'
import { Brain, Swords, BarChart3, Shield, Blocks, LineChart } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface FeaturesSectionProps {
  language: Language
}

export default function FeaturesSection({ language }: FeaturesSectionProps) {
  const features = [
    {
      icon: Brain,
      key: 'orchestration',
      highlight: true,
      showBadge: true,
    },
    {
      icon: Swords,
      key: 'arena',
      highlight: true,
      showBadge: true,
    },
    {
      icon: LineChart,
      key: 'data',
      highlight: true,
      showBadge: true,
    },
    {
      icon: Blocks,
      key: 'exchanges',
    },
    {
      icon: BarChart3,
      key: 'dashboard',
    },
    {
      icon: Shield,
      key: 'openSource',
    },
  ]

  return (
    <section className="py-24 relative" style={{ background: '#0B0E11' }}>
      {/* Background */}
      <div
        className="absolute inset-0 opacity-[0.02]"
        style={{
          backgroundImage: `linear-gradient(#F0B90B 1px, transparent 1px), linear-gradient(90deg, #F0B90B 1px, transparent 1px)`,
          backgroundSize: '40px 40px',
        }}
      />

      <div className="max-w-6xl mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          className="text-center mb-16"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="text-4xl lg:text-5xl font-bold mb-4" style={{ color: '#EAECEF' }}>
            {t('whyChooseNofx', language)}
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: '#848E9C' }}>
            {t('featuresSection.subtitle', language)}
          </p>
        </motion.div>

        {/* Features Grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
          {features.map((feature, index) => {
            const badge = feature.showBadge ? t(`featuresSection.cards.${feature.key}.badge`, language) : ''
            const FeatureIcon = feature.icon
            return (
              <motion.div
                key={feature.key}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.1 }}
                className={`
                  relative group rounded-2xl p-6 transition-all duration-300
                  ${feature.highlight ? 'md:col-span-1 lg:col-span-1' : ''}
                `}
                style={{
                  background: feature.highlight
                    ? 'linear-gradient(135deg, rgba(240, 185, 11, 0.08) 0%, rgba(240, 185, 11, 0.02) 100%)'
                    : '#12161C',
                  border: feature.highlight
                    ? '1px solid rgba(240, 185, 11, 0.2)'
                    : '1px solid rgba(255, 255, 255, 0.06)',
                }}
              >
                {/* Badge */}
                {feature.showBadge && badge && (
                  <div
                    className="absolute top-4 right-4 px-2 py-1 rounded text-xs font-medium"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      color: '#F0B90B',
                    }}
                  >
                    {badge}
                  </div>
                )}

                {/* Icon */}
                <motion.div
                  className="w-12 h-12 rounded-xl flex items-center justify-center mb-4"
                  style={{
                    background: feature.highlight
                      ? 'rgba(240, 185, 11, 0.15)'
                      : 'rgba(240, 185, 11, 0.1)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                  whileHover={{ scale: 1.1, rotate: 5 }}
                >
                  <FeatureIcon
                    className="w-6 h-6"
                    style={{ color: '#F0B90B' }}
                  />
                </motion.div>

                {/* Text */}
                <h3
                  className="text-xl font-bold mb-3"
                  style={{ color: '#EAECEF' }}
                >
                  {t(`featuresSection.cards.${feature.key}.title`, language)}
                </h3>
                <p
                  className="text-sm leading-relaxed"
                  style={{ color: '#848E9C' }}
                >
                  {t(`featuresSection.cards.${feature.key}.desc`, language)}
                </p>

                {/* Hover Glow */}
                <div
                  className="absolute -bottom-10 -right-10 w-32 h-32 rounded-full blur-3xl opacity-0 group-hover:opacity-30 transition-opacity duration-500"
                  style={{ background: '#F0B90B' }}
                />
              </motion.div>
            )
          })}
        </div>

        {/* Bottom Stats */}
        <motion.div
          className="mt-16 grid grid-cols-2 md:grid-cols-4 gap-6"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          {[
            { value: '10+', label: t('landingStats.aiModels', language) },
            { value: '5+', label: t('landingStats.exchanges', language) },
            { value: '24/7', label: t('landingStats.autoTrading', language) },
            { value: '100%', label: t('landingStats.openSource', language) },
          ].map((stat) => (
            <div
              key={stat.label}
              className="text-center p-4 rounded-xl"
              style={{
                background: 'rgba(255, 255, 255, 0.02)',
                border: '1px solid rgba(255, 255, 255, 0.05)',
              }}
            >
              <div
                className="text-2xl font-bold mb-1"
                style={{
                  background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                }}
              >
                {stat.value}
              </div>
              <div className="text-xs" style={{ color: '#5E6673' }}>
                {stat.label}
              </div>
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  )
}
