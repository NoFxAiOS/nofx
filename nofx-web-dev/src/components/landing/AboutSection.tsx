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
    <AnimatedSection id="about">
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
                style={{ color: 'var(--brand-red)' }}
              />
              <span
                className="text-sm font-semibold"
                style={{ color: 'var(--brand-red)' }}
              >
                {t('aboutNofx', language)}
              </span>
            </motion.div>

            <h2
              className="text-4xl font-bold"
              style={{ color: 'var(--text-primary)' }}
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
                style={{ background: 'rgba(229, 0, 18, 0.1)' }}
              >
                <Shield
                  className="w-6 h-6"
                  style={{ color: 'var(--brand-red)' }}
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
                background: '#1a1a1a',
                border: '1px solid var(--panel-border)',
              }}
            >
              <Typewriter
                lines={
                  language === 'zh'
                    ? [
                        '🤖 智慧云数 AI 分析中...',
                        '📊 步骤1/4：分析现有持仓 (BTC多头盈利+8.5%)',
                        '⚖️ 步骤2/4：评估账户风险 (保证金使用率30%，安全)',
                        '🎯 步骤3/4：筛选新机会 (ETH突破关键阻力位)',
                        '✅ 步骤4/4：决策完成 - 开多ETH，杠杆5x',
                        '📈 订单已提交，实时监控中...',
                      ]
                    : [
                        '🤖 智慧云数 AI Analysis in progress...',
                        '📊 Step 1/4: Analyzing positions (BTC long +8.5% profit)',
                        '⚖️ Step 2/4: Risk assessment (30% margin usage, safe)',
                        '🎯 Step 3/4: Finding opportunities (ETH breakout detected)',
                        '✅ Step 4/4: Decision complete - Open ETH long, 5x leverage',
                        '📈 Order submitted, monitoring in real-time...',
                      ]
                }
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
