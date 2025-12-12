import { motion } from 'framer-motion'
import { Download, Rocket, TrendingUp, AlertTriangle } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface HowItWorksSectionProps {
  language: Language
}

export default function HowItWorksSection({ language }: HowItWorksSectionProps) {
  const steps = [
    {
      icon: Download,
      number: '01',
      title: language === 'zh' ? '一键部署' : 'One-Click Deploy',
      desc: language === 'zh'
        ? '在你的服务器上运行一条命令即可完成部署'
        : 'Run a single command on your server to deploy',
      code: 'curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash',
    },
    {
      icon: Rocket,
      number: '02',
      title: language === 'zh' ? '访问面板' : 'Access Dashboard',
      desc: language === 'zh'
        ? '通过浏览器访问你的服务器'
        : 'Access your server via browser',
      code: 'http://YOUR_SERVER_IP:3000',
    },
    {
      icon: TrendingUp,
      number: '03',
      title: language === 'zh' ? '开始交易' : 'Start Trading',
      desc: language === 'zh'
        ? '创建交易员，让 AI 开始工作'
        : 'Create trader, let AI do the work',
      code: language === 'zh' ? '配置模型 → 配置交易所 → 创建交易员' : 'Configure Model → Exchange → Create Trader',
    },
  ]

  return (
    <section className="py-24 relative overflow-hidden" style={{ background: '#0D1117' }}>
      {/* Background Decoration */}
      <div
        className="absolute left-0 top-1/2 -translate-y-1/2 w-96 h-96 rounded-full blur-3xl opacity-20"
        style={{ background: 'radial-gradient(circle, rgba(240, 185, 11, 0.15) 0%, transparent 70%)' }}
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
            {t('howToStart', language)}
          </h2>
          <p className="text-lg" style={{ color: '#848E9C' }}>
            {t('fourSimpleSteps', language)}
          </p>
        </motion.div>

        {/* Steps Timeline */}
        <div className="relative">
          {/* Connecting Line */}
          <div
            className="absolute left-[39px] top-0 bottom-0 w-px hidden lg:block"
            style={{ background: 'linear-gradient(to bottom, transparent, rgba(240, 185, 11, 0.3), transparent)' }}
          />

          <div className="space-y-6">
            {steps.map((step, index) => (
              <motion.div
                key={step.number}
                initial={{ opacity: 0, x: -30 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.15 }}
                className="relative"
              >
                <div
                  className="flex flex-col lg:flex-row items-start gap-6 p-6 rounded-2xl transition-all duration-300 hover:translate-x-2"
                  style={{
                    background: 'rgba(255, 255, 255, 0.02)',
                    border: '1px solid rgba(255, 255, 255, 0.05)',
                  }}
                >
                  {/* Number Circle */}
                  <div className="flex-shrink-0 relative z-10">
                    <motion.div
                      className="w-20 h-20 rounded-2xl flex items-center justify-center"
                      style={{
                        background: 'linear-gradient(135deg, rgba(240, 185, 11, 0.2) 0%, rgba(240, 185, 11, 0.05) 100%)',
                        border: '1px solid rgba(240, 185, 11, 0.3)',
                      }}
                      whileHover={{ scale: 1.1 }}
                    >
                      <step.icon className="w-8 h-8" style={{ color: '#F0B90B' }} />
                    </motion.div>
                  </div>

                  {/* Content */}
                  <div className="flex-grow">
                    <div className="flex items-center gap-3 mb-2">
                      <span
                        className="text-sm font-mono font-bold"
                        style={{ color: '#F0B90B' }}
                      >
                        {step.number}
                      </span>
                      <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                        {step.title}
                      </h3>
                    </div>
                    <p className="mb-4" style={{ color: '#848E9C' }}>
                      {step.desc}
                    </p>

                    {/* Code Block */}
                    <div
                      className="inline-flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-sm"
                      style={{
                        background: 'rgba(0, 0, 0, 0.3)',
                        border: '1px solid rgba(255, 255, 255, 0.06)',
                      }}
                    >
                      <span style={{ color: '#5E6673' }}>$</span>
                      <span style={{ color: '#EAECEF' }}>{step.code}</span>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </div>
        </div>

        {/* Risk Warning */}
        <motion.div
          className="mt-12 p-6 rounded-2xl flex items-start gap-4"
          style={{
            background: 'rgba(240, 185, 11, 0.05)',
            border: '1px solid rgba(240, 185, 11, 0.15)',
          }}
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <div
            className="w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0"
            style={{ background: 'rgba(240, 185, 11, 0.1)' }}
          >
            <AlertTriangle className="w-6 h-6" style={{ color: '#F0B90B' }} />
          </div>
          <div>
            <div className="font-semibold mb-2" style={{ color: '#F0B90B' }}>
              {t('importantRiskWarning', language)}
            </div>
            <p className="text-sm leading-relaxed" style={{ color: '#5E6673' }}>
              {t('riskWarningText', language)}
            </p>
          </div>
        </motion.div>
      </div>
    </section>
  )
}
