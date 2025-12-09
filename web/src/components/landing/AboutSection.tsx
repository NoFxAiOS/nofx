import { motion } from 'framer-motion'
import { Terminal, Shield, Cpu, BarChart3 } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface AboutSectionProps {
  language: Language
}

export default function AboutSection({ language }: AboutSectionProps) {
  const features = [
    {
      icon: Shield,
      title: language === 'zh' ? '完全自主控制' : 'Full Control',
      desc: language === 'zh' ? '自托管，数据安全' : 'Self-hosted, data secure',
    },
    {
      icon: Cpu,
      title: language === 'zh' ? '多 AI 支持' : 'Multi-AI Support',
      desc: language === 'zh' ? 'DeepSeek, GPT, Claude...' : 'DeepSeek, GPT, Claude...',
    },
    {
      icon: BarChart3,
      title: language === 'zh' ? '实时监控' : 'Real-time Monitor',
      desc: language === 'zh' ? '可视化交易看板' : 'Visual trading dashboard',
    },
  ]

  return (
    <section className="py-24 relative overflow-hidden" style={{ background: '#0B0E11' }}>
      {/* Background Decoration */}
      <div
        className="absolute top-0 right-0 w-96 h-96 rounded-full blur-3xl opacity-30"
        style={{ background: 'radial-gradient(circle, rgba(240, 185, 11, 0.1) 0%, transparent 70%)' }}
      />

      <div className="max-w-6xl mx-auto px-4">
        <div className="grid lg:grid-cols-2 gap-16 items-center">
          {/* Left Content */}
          <motion.div
            initial={{ opacity: 0, x: -30 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6 }}
          >
            <motion.div
              className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full mb-6"
              style={{
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
            >
              <Terminal className="w-4 h-4" style={{ color: '#F0B90B' }} />
              <span className="text-xs font-medium" style={{ color: '#F0B90B' }}>
                {t('aboutNofx', language)}
              </span>
            </motion.div>

            <h2 className="text-4xl lg:text-5xl font-bold mb-6" style={{ color: '#EAECEF' }}>
              {t('whatIsNofx', language)}
            </h2>

            <p className="text-lg mb-8 leading-relaxed" style={{ color: '#848E9C' }}>
              {t('nofxNotAnotherBot', language)} {t('nofxDescription1', language)}
            </p>

            {/* Feature Pills */}
            <div className="flex flex-wrap gap-3">
              {features.map((feature, index) => (
                <motion.div
                  key={feature.title}
                  initial={{ opacity: 0, y: 20 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  viewport={{ once: true }}
                  transition={{ delay: index * 0.1 }}
                  className="flex items-center gap-3 px-4 py-3 rounded-xl"
                  style={{
                    background: 'rgba(255, 255, 255, 0.03)',
                    border: '1px solid rgba(255, 255, 255, 0.06)',
                  }}
                >
                  <div
                    className="w-10 h-10 rounded-lg flex items-center justify-center"
                    style={{ background: 'rgba(240, 185, 11, 0.1)' }}
                  >
                    <feature.icon className="w-5 h-5" style={{ color: '#F0B90B' }} />
                  </div>
                  <div>
                    <div className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                      {feature.title}
                    </div>
                    <div className="text-xs" style={{ color: '#5E6673' }}>
                      {feature.desc}
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>
          </motion.div>

          {/* Right - Terminal */}
          <motion.div
            initial={{ opacity: 0, x: 30 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6, delay: 0.2 }}
          >
            <div
              className="rounded-2xl overflow-hidden"
              style={{
                background: '#0D1117',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)',
              }}
            >
              {/* Terminal Header */}
              <div
                className="flex items-center gap-2 px-4 py-3"
                style={{ background: 'rgba(255, 255, 255, 0.03)', borderBottom: '1px solid rgba(255, 255, 255, 0.06)' }}
              >
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full" style={{ background: '#FF5F56' }} />
                  <div className="w-3 h-3 rounded-full" style={{ background: '#FFBD2E' }} />
                  <div className="w-3 h-3 rounded-full" style={{ background: '#27C93F' }} />
                </div>
                <span className="text-xs ml-2" style={{ color: '#5E6673' }}>terminal</span>
              </div>

              {/* Terminal Content */}
              <div className="p-6 font-mono text-sm space-y-2">
                <div style={{ color: '#5E6673' }}>$ git clone https://github.com/NoFxAiOS/nofx.git</div>
                <div style={{ color: '#5E6673' }}>$ cd nofx && chmod +x start.sh</div>
                <div style={{ color: '#5E6673' }}>$ ./start.sh start --build</div>
                <div className="pt-2" style={{ color: '#F0B90B' }}>
                  ✓ {t('startupMessages1', language)}
                </div>
                <div style={{ color: '#0ECB81' }}>
                  ✓ {t('startupMessages2', language)}
                </div>
                <div style={{ color: '#0ECB81' }}>
                  ✓ {t('startupMessages3', language)}
                </div>
                <motion.div
                  className="flex items-center gap-2 pt-2"
                  animate={{ opacity: [1, 0.5, 1] }}
                  transition={{ duration: 1.5, repeat: Infinity }}
                >
                  <span style={{ color: '#F0B90B' }}>▸</span>
                  <span style={{ color: '#EAECEF' }}>_</span>
                </motion.div>
              </div>
            </div>
          </motion.div>
        </div>
      </div>
    </section>
  )
}
