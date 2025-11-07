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
    <AnimatedSection id="about" backgroundColor="var(--brand-dark-gray)">
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
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
              whileHover={{ scale: 1.05 }}
            >
              <Target
                className="w-4 h-4"
                style={{ color: 'var(--brand-yellow)' }}
              />
              <span
                className="text-sm font-semibold"
                style={{ color: 'var(--brand-yellow)' }}
              >
                {t('aboutNofx', language)}
              </span>
            </motion.div>

            <h2
              className="text-4xl font-bold"
              style={{ color: 'var(--brand-light-gray)' }}
            >
              {t('whatIsNofx', language)}
            </h2>
            <p
              className="text-lg leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('nofxNotAnotherBot', language)}{' '}
              {t('nofxDescription1', language)}{' '}
              {t('nofxDescription2', language)}
            </p>
            <p
              className="text-lg leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('nofxDescription3', language)}{' '}
              {t('nofxDescription4', language)}{' '}
              {t('nofxDescription5', language)}
            </p>
            <motion.div
              className="flex items-center gap-3 pt-4"
              whileHover={{ x: 5 }}
            >
              <div
                className="w-12 h-12 rounded-full flex items-center justify-center"
                style={{ background: 'rgba(240, 185, 11, 0.1)' }}
              >
                <Shield
                  className="w-6 h-6"
                  style={{ color: 'var(--brand-yellow)' }}
                />
              </div>
              <div>
                <div
                  className="font-semibold"
                  style={{ color: 'var(--brand-light-gray)' }}
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
                background: 'var(--brand-black)',
                border: '1px solid var(--panel-border)',
              }}
            >
              <Typewriter
                lines={[
                  '$ git clone https://github.com/tinkle-community/nofx.git',
                  '$ cd nofx',
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
