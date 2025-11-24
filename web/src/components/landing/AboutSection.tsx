import { motion } from 'framer-motion'
import { Shield, Target } from 'lucide-react'
import AnimatedSection from './AnimatedSection'
import Typewriter from '../Typewriter'
import { t, Language } from '../../i18n/translations'

interface AboutSectionProps {
  language: Language
}

export default function AboutSection({ language }: AboutSectionProps) {
  return (
    <AnimatedSection id="about" backgroundColor="var(--panel-bg)">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-2 gap-12 items-center">
          <motion.div
            className="space-y-6"
            initial={{ opacity: 0, x: -50 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6 }}
          >
            <motion.div
              className="inline-flex items-center gap-2 px-4 py-2 rounded-full"
              style={{
                background: 'rgba(229, 0, 18, 0.1)',
                border: '1px solid rgba(229, 0, 18, 0.2)',
              }}
              whileHover={{ scale: 1.05 }}
            >
              <Target
                className="w-4 h-4"
                style={{ color: 'var(--accent-red)' }}
              />
              <span
                className="text-sm font-semibold"
                style={{ color: 'var(--accent-red)' }}
              >
                {t('aboutAres', language)}
              </span>
            </motion.div>

            <h2
              className="text-4xl font-bold"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('whatIsAres', language)}
            </h2>
            <p
              className="text-lg leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('aresNotAnotherBot', language)}{' '}
              {t('aresDescription1', language)}{' '}
              {t('aresDescription2', language)}
            </p>
            <p
              className="text-lg leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('aresDescription3', language)}{' '}
              {t('aresDescription4', language)}{' '}
              {t('aresDescription5', language)}
            </p>
            <motion.div
              className="flex items-center gap-3 pt-4"
              whileHover={{ x: 5 }}
            >
              <div
                className="w-12 h-12 rounded-full flex items-center justify-center"
                style={{ background: 'rgba(229, 0, 18, 0.1)' }}
              >
                <Shield
                  className="w-6 h-6"
                  style={{ color: 'var(--accent-red)' }}
                />
              </div>
              <div>
                <div
                  className="font-semibold"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('youFullControl', language)}
                </div>
                <div
                  className="text-sm"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('fullControlDesc', language)}
                </div>
              </div>
            </motion.div>
          </motion.div>

          <div className="relative">
            <div
              className="rounded-2xl p-8"
              style={{
                background: 'var(--panel-bg)',
                border: '1px solid var(--panel-border)',
              }}
            >
              <Typewriter
                lines={[
                  '$ cd ares',
                  '$ chmod +x start.sh',
                  '$ ./start.sh start --build',
                  t('startupMessages1', language),
                  t('startupMessages2', language),
                  t('startupMessages3', language),
                ]}
                typingSpeed={70}
                lineDelay={900}
                className="text-sm font-mono"
                style={{
                  color: '#00FF88',
                  textShadow: '0 0 8px rgba(0,255,136,0.4)',
                }}
              />
            </div>
          </div>
        </div>
      </div>
    </AnimatedSection>
  )
}
