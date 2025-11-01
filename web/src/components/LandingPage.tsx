import { useState, useRef } from 'react'
import { motion, useScroll, useTransform, useInView } from 'framer-motion'
import {
  Menu,
  X,
  Code,
  GitBranch,
  Shield,
  ArrowRight,
  Sparkles,
  Target,
  Rocket,
  Lock,
  Cpu,
} from 'lucide-react'
import { CryptoFeatureCard } from './CryptoFeatureCard'
import Typewriter from './Typewriter'

// Animation variants
const fadeInUp = {
  initial: { opacity: 0, y: 60 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.6, ease: [0.6, -0.05, 0.01, 0.99] },
}

const fadeInScale = {
  initial: { opacity: 0, scale: 0.8 },
  animate: { opacity: 1, scale: 1 },
  transition: { duration: 0.5 },
}

const staggerContainer = {
  animate: {
    transition: {
      staggerChildren: 0.1,
    },
  },
}

const floatingAnimation = {
  y: [0, -20, 0],
  transition: {
    duration: 3,
    repeat: Infinity,
  },
}

export function LandingPage() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [showLoginModal, setShowLoginModal] = useState(false)
  const { scrollYProgress } = useScroll()
  const opacity = useTransform(scrollYProgress, [0, 0.2], [1, 0])
  const scale = useTransform(scrollYProgress, [0, 0.2], [1, 0.8])

  return (
    <div
      className='min-h-screen overflow-hidden'
      style={{
        background: 'var(--brand-black)',
        color: 'var(--brand-light-gray)',
      }}
    >

      {/* Navbar */}
      <nav className='fixed top-0 w-full z-50 header-bar'>
        <div className='max-w-7xl mx-auto px-4 sm:px-6 lg:px-8'>
          <div className='flex items-center justify-between h-16'>
            {/* Logo */}
            <div className='flex items-center gap-3'>
              <img src='/images/logo.png' alt='NOFX Logo' className='w-8 h-8' />
              <span
                className='text-xl font-bold'
                style={{ color: 'var(--brand-yellow)' }}
              >
                NOFX
              </span>
              <span
                className='text-sm hidden sm:block'
                style={{ color: 'var(--text-secondary)' }}
              >
                Agentic Trading OS
              </span>
            </div>

            {/* Desktop Menu */}
            <div className='hidden md:flex items-center gap-6'>
              {['功能', '如何运作', 'GitHub', '社区'].map((item) => (
                <a
                  key={item}
                  href={
                    item === 'GitHub'
                      ? 'https://github.com/tinkle-community/nofx'
                      : item === '社区'
                      ? 'https://t.me/nofx_dev_community'
                      : `#${item === '功能' ? 'features' : 'how-it-works'}`
                  }
                  target={
                    item === 'GitHub' || item === '社区' ? '_blank' : undefined
                  }
                  rel={
                    item === 'GitHub' || item === '社区'
                      ? 'noopener noreferrer'
                      : undefined
                  }
                  className='text-sm transition-colors relative group'
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {item}
                  <span
                    className='absolute -bottom-1 left-0 w-0 h-0.5 group-hover:w-full transition-all duration-300'
                    style={{ background: 'var(--brand-yellow)' }}
                  />
                </a>
              ))}
              <button
                onClick={() => setShowLoginModal(true)}
                className='px-4 py-2 rounded font-semibold text-sm'
                style={{
                  background: 'var(--brand-yellow)',
                  color: 'var(--brand-black)',
                }}
              >
                登录 / 注册
              </button>
            </div>

            {/* Mobile Menu Button */}
            <motion.button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className='md:hidden'
              style={{ color: 'var(--brand-light-gray)' }}
              whileTap={{ scale: 0.9 }}
            >
              {mobileMenuOpen ? (
                <X className='w-6 h-6' />
              ) : (
                <Menu className='w-6 h-6' />
              )}
            </motion.button>
          </div>
        </div>

        {/* Mobile Menu */}
        <motion.div
          initial={false}
          animate={
            mobileMenuOpen
              ? { height: 'auto', opacity: 1 }
              : { height: 0, opacity: 0 }
          }
          transition={{ duration: 0.3 }}
          className='md:hidden overflow-hidden'
          style={{
            background: 'var(--brand-dark-gray)',
            borderTop: '1px solid rgba(240, 185, 11, 0.1)',
          }}
        >
          <div className='px-4 py-4 space-y-3'>
            {['功能', '如何运作', 'GitHub', '社区'].map((item) => (
              <a
                key={item}
                href={`#${item}`}
                className='block text-sm py-2'
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {item}
              </a>
            ))}
            <button
              onClick={() => {
                setShowLoginModal(true)
                setMobileMenuOpen(false)
              }}
              className='w-full px-4 py-2 rounded font-semibold text-sm mt-2'
              style={{
                background: 'var(--brand-yellow)',
                color: 'var(--brand-black)',
              }}
            >
              登录 / 注册
            </button>
          </div>
        </motion.div>
      </nav>

      {/* Hero Section */}
      <section className='relative pt-32 pb-20 px-4'>
        <div className='max-w-7xl mx-auto'>
          <div className='grid lg:grid-cols-2 gap-12 items-center'>
            {/* Left Content */}
            <motion.div
              className='space-y-6 relative z-10'
              style={{ opacity, scale }}
              initial='initial'
              animate='animate'
              variants={staggerContainer}
            >
              <motion.div variants={fadeInUp}>
                <motion.div
                  className='inline-flex items-center gap-2 px-4 py-2 rounded-full mb-6'
                  style={{
                    background: 'rgba(240, 185, 11, 0.1)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                  whileHover={{
                    scale: 1.05,
                    boxShadow: '0 0 20px rgba(240, 185, 11, 0.2)',
                  }}
                >
                  <Sparkles
                    className='w-4 h-4'
                    style={{ color: 'var(--brand-yellow)' }}
                  />
                  <span
                    className='text-sm font-semibold'
                    style={{ color: 'var(--brand-yellow)' }}
                  >
                    3 天内 2.5K+ GitHub Stars
                  </span>
                </motion.div>
              </motion.div>

              <h1
                className='text-5xl lg:text-7xl font-bold leading-tight'
                style={{ color: 'var(--brand-light-gray)' }}
              >
                Read the Market.
                <br />
                <span style={{ color: 'var(--brand-yellow)' }}>Write the Trade.</span>
              </h1>

              <motion.p
                className='text-xl leading-relaxed'
                style={{ color: 'var(--text-secondary)' }}
                variants={fadeInUp}
              >
                NOFX 是 AI
                交易的未来标准——一个开放、社区驱动的代理式交易操作系统。支持
                Binance、Aster DEX 等交易所，自托管、多代理竞争，让 AI
                为你自动决策、执行和优化交易。
              </motion.p>

              <div className='flex items-center gap-3 flex-wrap'>
                <motion.a
                  href='https://github.com/tinkle-community/nofx'
                  target='_blank'
                  rel='noopener noreferrer'
                  whileHover={{ scale: 1.05 }}
                  transition={{ type: 'spring', stiffness: 400 }}
                >
                  <img
                    src='https://img.shields.io/github/stars/tinkle-community/nofx?style=for-the-badge&logo=github&logoColor=white&color=F0B90B&labelColor=1E2329'
                    alt='GitHub Stars'
                    className='h-7'
                  />
                </motion.a>
                <motion.a
                  href='https://github.com/tinkle-community/nofx/network/members'
                  target='_blank'
                  rel='noopener noreferrer'
                  whileHover={{ scale: 1.05 }}
                  transition={{ type: 'spring', stiffness: 400 }}
                >
                  <img
                    src='https://img.shields.io/github/forks/tinkle-community/nofx?style=for-the-badge&logo=github&logoColor=white&color=F0B90B&labelColor=1E2329'
                    alt='GitHub Forks'
                    className='h-7'
                  />
                </motion.a>
                <motion.a
                  href='https://github.com/tinkle-community/nofx/graphs/contributors'
                  target='_blank'
                  rel='noopener noreferrer'
                  whileHover={{ scale: 1.05 }}
                  transition={{ type: 'spring', stiffness: 400 }}
                >
                  <img
                    src='https://img.shields.io/github/contributors/tinkle-community/nofx?style=for-the-badge&logo=github&logoColor=white&color=F0B90B&labelColor=1E2329'
                    alt='GitHub Contributors'
                    className='h-7'
                  />
                </motion.a>
              </div>

              <motion.p
                className='text-xs pt-4'
                style={{ color: 'var(--text-tertiary)' }}
                variants={fadeInUp}
              >
                由 Aster DEX 和 Binance 提供支持，Amber.ac 战略投资。
              </motion.p>
            </motion.div>

            {/* Right Content - Visual */}
            <motion.div
              className='relative'
              initial={{ opacity: 0, x: 100 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.8, delay: 0.2 }}
            >
              <motion.div
                className='rounded-2xl p-8 relative z-10'
                style={{
                  background: 'var(--panel-bg)',
                  border: '1px solid var(--panel-border)',
                }}
                animate={floatingAnimation}
              >
                <motion.img
                  src='/images/main.png'
                  alt='NOFX Platform'
                  className='w-full opacity-90'
                  whileHover={{ scale: 1.05, rotate: 5 }}
                  transition={{ type: 'spring', stiffness: 300 }}
                />
              </motion.div>

            </motion.div>
          </div>
        </div>

        {/* Scroll Indicator removed for cleaner Binance-like style */}
      </section>

      {/* About Section */}
      <AnimatedSection id='about' backgroundColor='var(--brand-dark-gray)'>
        <div className='max-w-7xl mx-auto'>
          <div className='grid lg:grid-cols-2 gap-12 items-center'>
            <motion.div
              className='space-y-6'
              initial={{ opacity: 0, x: -50 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.6 }}
            >
              <motion.div
                className='inline-flex items-center gap-2 px-4 py-2 rounded-full'
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  border: '1px solid rgba(240, 185, 11, 0.2)',
                }}
                whileHover={{ scale: 1.05 }}
              >
                <Target
                  className='w-4 h-4'
                  style={{ color: 'var(--brand-yellow)' }}
                />
                <span
                  className='text-sm font-semibold'
                  style={{ color: 'var(--brand-yellow)' }}
                >
                  关于 NOFX
                </span>
              </motion.div>

              <h2
                className='text-4xl font-bold'
                style={{ color: 'var(--brand-light-gray)' }}
              >
                什么是 NOFX？
              </h2>
              <p
                className='text-lg leading-relaxed'
                style={{ color: 'var(--text-secondary)' }}
              >
                NOFX 不是另一个交易机器人，而是 AI 交易的 'Linux' ——
                一个透明、可信任的开源 OS，提供统一的 '决策-风险-执行'
                层，支持所有资产类别。
              </p>
              <p
                className='text-lg leading-relaxed'
                style={{ color: 'var(--text-secondary)' }}
              >
                从加密市场起步（24/7、高波动性完美测试场），未来扩展到股票、期货、外汇。核心：开放架构、AI
                达尔文主义（多代理自竞争、策略进化）、CodeFi 飞轮（开发者 PR
                贡献获积分奖励）。
              </p>
              <motion.div
                className='flex items-center gap-3 pt-4'
                whileHover={{ x: 5 }}
              >
                <div
                  className='w-12 h-12 rounded-full flex items-center justify-center'
                  style={{ background: 'rgba(240, 185, 11, 0.1)' }}
                >
                  <Shield
                    className='w-6 h-6'
                    style={{ color: 'var(--brand-yellow)' }}
                  />
                </div>
                <div>
                  <div
                    className='font-semibold'
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    你 100% 掌控
                  </div>
                  <div
                    className='text-sm'
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    完全掌控 AI 提示词和资金
                  </div>
                </div>
              </motion.div>
            </motion.div>

            <div className='relative'>
              <div
                className='rounded-2xl p-8'
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
                    '🚀 启动自动交易系统...',
                    '✓ API服务器启动在端口 8080',
                    '🌐 Web 控制台 http://localhost:3000',
                  ]}
                  typingSpeed={70}
                  lineDelay={900}
                  className='text-sm font-mono'
                  style={{ color: '#00FF41', textShadow: '0 0 6px rgba(0,255,65,0.6)' }}
                />
              </div>
            </div>
          </div>
        </div>
      </AnimatedSection>

      {/* Features Section */}
      <AnimatedSection id='features'>
        <div className='max-w-7xl mx-auto'>
          <motion.div
            className='text-center mb-16'
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
          >
            <motion.div
              className='inline-flex items-center gap-2 px-4 py-2 rounded-full mb-6'
              style={{
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
              whileHover={{ scale: 1.05 }}
            >
              <Rocket
                className='w-4 h-4'
                style={{ color: 'var(--brand-yellow)' }}
              />
              <span
                className='text-sm font-semibold'
                style={{ color: 'var(--brand-yellow)' }}
              >
                核心功能
              </span>
            </motion.div>
            <h2
              className='text-4xl font-bold mb-4'
              style={{ color: 'var(--brand-light-gray)' }}
            >
              为什么选择 NOFX？
            </h2>
            <p className='text-lg' style={{ color: 'var(--text-secondary)' }}>
              开源、透明、社区驱动的 AI 交易操作系统
            </p>
          </motion.div>

          <div className='grid md:grid-cols-2 lg:grid-cols-3 gap-8 max-w-7xl mx-auto'>
            <CryptoFeatureCard
              icon={<Code className='w-8 h-8' />}
              title='100% 开源与自托管'
              description='你的框架，你的规则。非黑箱，支持自定义提示词和多模型。'
              features={[
                '完全开源代码',
                '支持自托管部署',
                '自定义 AI 提示词',
                '多模型支持（DeepSeek、Qwen）',
              ]}
              delay={0}
            />
            <CryptoFeatureCard
              icon={<Cpu className='w-8 h-8' />}
              title='多代理智能竞争'
              description='AI 策略在沙盒中高速战斗，最优者生存，实现策略进化。'
              features={[
                '多 AI 代理并行运行',
                '策略自动优化',
                '沙盒安全测试',
                '跨市场策略移植',
              ]}
              delay={0.1}
            />
            <CryptoFeatureCard
              icon={<Lock className='w-8 h-8' />}
              title='安全可靠交易'
              description='企业级安全保障，完全掌控你的资金和交易策略。'
              features={[
                '本地私钥管理',
                'API 权限精细控制',
                '实时风险监控',
                '交易日志审计',
              ]}
              delay={0.2}
            />
          </div>
        </div>
      </AnimatedSection>

      {/* How It Works Section */}
      <AnimatedSection
        id='how-it-works'
        backgroundColor='var(--brand-dark-gray)'
      >
        <div className='max-w-7xl mx-auto'>
          <motion.div
            className='text-center mb-16'
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
          >
            <h2
              className='text-4xl font-bold mb-4'
              style={{ color: 'var(--brand-light-gray)' }}
            >
              如何开始使用 NOFX
            </h2>
            <p className='text-lg' style={{ color: 'var(--text-secondary)' }}>
              四个简单步骤，开启 AI 自动交易之旅
            </p>
          </motion.div>

          <div className='space-y-8'>
            {[
              {
                number: 1,
                title: '拉取 GitHub 仓库',
                description:
                  'git clone https://github.com/tinkle-community/nofx 并切换到 dev 分支测试新功能。',
              },
              {
                number: 2,
                title: '配置环境',
                description:
                  '前端设置交易所 API（如 Binance、Hyperliquid）、AI 模型和自定义提示词。',
              },
              {
                number: 3,
                title: '部署与运行',
                description:
                  '一键 Docker 部署，启动 AI 代理。注意：高风险市场，仅用闲钱测试。',
              },
              {
                number: 4,
                title: '优化与贡献',
                description:
                  '监控交易，提交 PR 改进框架。加入 Telegram 分享策略。',
              },
            ].map((step, index) => (
              <StepCard key={step.number} {...step} delay={index * 0.1} />
            ))}
          </div>

          <motion.div
            className='mt-12 p-6 rounded-xl flex items-start gap-4'
            style={{
              background: 'rgba(246, 70, 93, 0.1)',
              border: '1px solid rgba(246, 70, 93, 0.3)',
            }}
            initial={{ opacity: 0, scale: 0.9 }}
            whileInView={{ opacity: 1, scale: 1 }}
            viewport={{ once: true }}
            whileHover={{ scale: 1.02 }}
          >
            <div
              className='w-10 h-10 rounded-full flex items-center justify-center flex-shrink-0'
              style={{ background: 'rgba(246, 70, 93, 0.2)' }}
            >
              <span className='text-xl'>⚠️</span>
            </div>
            <div>
              <div className='font-semibold mb-2' style={{ color: '#F6465D' }}>
                重要风险提示
              </div>
              <p className='text-sm' style={{ color: 'var(--text-secondary)' }}>
                dev 分支不稳定，勿用无法承受损失的资金。NOFX
                非托管，无官方策略。交易有风险，投资需谨慎。
              </p>
            </div>
          </motion.div>
        </div>
      </AnimatedSection>

      {/* Community Section */}
      <AnimatedSection>
        <div className='max-w-7xl mx-auto'>
          <motion.div
            className='grid md:grid-cols-3 gap-6'
            variants={staggerContainer}
            initial='initial'
            whileInView='animate'
            viewport={{ once: true }}
          >
            <TestimonialCard
              quote='跑了一晚上 NOFX，开源的 AI 自动交易，太有意思了，一晚上赚了 6% 收益！'
              author='@DIYgod'
              delay={0}
            />
            <TestimonialCard
              quote='所有成功人士都在用 NOFX。IYKYK。'
              author='@SexyMichill'
              delay={0.1}
            />
            <TestimonialCard
              quote='NOFX 复兴了传奇 Alpha Arena，AI 驱动的加密期货战场。'
              author='@hqmank'
              delay={0.2}
            />
          </motion.div>
        </div>
      </AnimatedSection>

      {/* CTA Section */}
      <AnimatedSection backgroundColor='var(--panel-bg)'>
        <div className='max-w-4xl mx-auto text-center'>
          <motion.h2
            className='text-5xl font-bold mb-6'
            style={{ color: 'var(--brand-light-gray)' }}
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
          >
            准备好定义 AI 交易的未来吗？
          </motion.h2>
          <motion.p
            className='text-xl mb-12'
            style={{ color: 'var(--text-secondary)' }}
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: 0.1 }}
          >
            从加密市场起步，扩展到 TradFi。NOFX 是 AgentFi 的基础架构。
          </motion.p>
          <motion.div
            className='flex flex-wrap justify-center gap-4'
            variants={staggerContainer}
            initial='initial'
            whileInView='animate'
            viewport={{ once: true }}
          >
            <motion.button
              onClick={() => setShowLoginModal(true)}
              className='flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg'
              style={{
                background: 'var(--brand-yellow)',
                color: 'var(--brand-black)',
              }}
              variants={fadeInScale}
              whileHover={{
                scale: 1.05,
                boxShadow: '0 20px 60px rgba(240, 185, 11, 0.4)',
              }}
              whileTap={{ scale: 0.95 }}
            >
              <Rocket className='w-6 h-6' />
              立即开始
              <motion.div
                animate={{ x: [0, 5, 0] }}
                transition={{ duration: 1.5, repeat: Infinity }}
              >
                <ArrowRight className='w-5 h-5' />
              </motion.div>
            </motion.button>
            <motion.a
              href='https://github.com/tinkle-community/nofx/tree/dev'
              target='_blank'
              rel='noopener noreferrer'
              className='flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg'
              style={{
                background: 'var(--brand-dark-gray)',
                color: 'var(--brand-light-gray)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
              variants={fadeInScale}
              whileHover={{
                scale: 1.05,
                borderColor: 'var(--brand-yellow)',
                boxShadow: '0 20px 60px rgba(240, 185, 11, 0.2)',
              }}
              whileTap={{ scale: 0.95 }}
            >
              <GitBranch className='w-6 h-6' />
              查看源码
            </motion.a>
          </motion.div>
        </div>
      </AnimatedSection>

      {/* Footer */}
      <footer
        style={{
          borderTop: '1px solid rgba(240, 185, 11, 0.1)',
          background: 'var(--brand-black)',
        }}
      >
        <div className='max-w-7xl mx-auto px-4 py-12'>
          <div className='grid md:grid-cols-4 gap-8 mb-8'>
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
            >
              <div className='flex items-center gap-2 mb-4'>
                <img
                  src='/images/logo.png'
                  alt='NOFX Logo'
                  className='w-8 h-8'
                />
                <span
                  className='text-xl font-bold'
                  style={{ color: 'var(--brand-yellow)' }}
                >
                  NOFX
                </span>
              </div>
              <p className='text-sm' style={{ color: 'var(--text-secondary)' }}>
                AI 交易的未来标准
              </p>
            </motion.div>
            {[
              {
                title: '链接',
                links: [
                  {
                    text: 'GitHub',
                    href: 'https://github.com/tinkle-community/nofx',
                  },
                  { text: 'Telegram', href: 'https://t.me/nofx_dev_community' },
                  { text: 'X (Twitter)', href: 'https://x.com/nofx_ai' },
                ],
              },
              {
                title: '资源',
                links: [
                  {
                    text: '文档',
                    href: 'https://github.com/tinkle-community/nofx#readme',
                  },
                  {
                    text: 'Issues',
                    href: 'https://github.com/tinkle-community/nofx/issues',
                  },
                  {
                    text: 'Pull Requests',
                    href: 'https://github.com/tinkle-community/nofx/pulls',
                  },
                ],
              },
              {
                title: '支持方',
                items: ['Aster DEX', 'Binance', 'Amber.ac (战略投资)'],
              },
            ].map((section, index) => (
              <motion.div
                key={section.title}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.1 }}
              >
                <h3
                  className='font-semibold mb-4'
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {section.title}
                </h3>
                <div className='space-y-2'>
                  {section.links
                    ? section.links.map((link) => (
                        <motion.a
                          key={link.text}
                          href={link.href}
                          target='_blank'
                          rel='noopener noreferrer'
                          className='block text-sm transition-colors'
                          style={{ color: 'var(--text-secondary)' }}
                          whileHover={{ color: 'var(--brand-yellow)', x: 5 }}
                        >
                          {link.text}
                        </motion.a>
                      ))
                    : section.items?.map((item) => (
                        <p
                          key={item}
                          className='text-sm'
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {item}
                        </p>
                      ))}
                </div>
              </motion.div>
            ))}
          </div>
          <motion.div
            className='pt-8 border-t text-center'
            style={{ borderColor: 'rgba(240, 185, 11, 0.1)' }}
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            viewport={{ once: true }}
          >
            <p
              className='text-sm mb-2'
              style={{ color: 'var(--text-tertiary)' }}
            >
              © 2025 NOFX. All rights reserved. Backed by Amber.ac.
            </p>
            <p className='text-xs' style={{ color: 'var(--text-tertiary)' }}>
              ⚠️ 风险警告：交易有风险，NOFX
              不提供投资建议。请在充分了解风险的情况下使用本系统。
            </p>
          </motion.div>
        </div>
      </footer>

      {/* Login Modal */}
      {showLoginModal && (
        <LoginModal onClose={() => setShowLoginModal(false)} />
      )}
    </div>
  )
}

function AnimatedSection({
  children,
  id,
  backgroundColor = 'var(--brand-black)',
}: any) {
  const ref = useRef(null)
  const isInView = useInView(ref, { once: true, margin: '-100px' })

  return (
    <motion.section
      id={id}
      ref={ref}
      className='py-20 px-4'
      style={{ background: backgroundColor }}
      initial={{ opacity: 0 }}
      animate={isInView ? { opacity: 1 } : { opacity: 0 }}
      transition={{ duration: 0.6 }}
    >
      {children}
    </motion.section>
  )
}

// Removed unused FeatureCard component

function StepCard({ number, title, description, delay }: any) {
  return (
    <motion.div
      className='flex gap-6 items-start'
      initial={{ opacity: 0, x: -50 }}
      whileInView={{ opacity: 1, x: 0 }}
      viewport={{ once: true }}
      transition={{ delay }}
      whileHover={{ x: 10 }}
    >
      <motion.div
        className='flex-shrink-0 w-14 h-14 rounded-full flex items-center justify-center font-bold text-2xl'
        style={{ background: 'var(--binance-yellow)', color: 'var(--brand-black)' }}
        whileHover={{ scale: 1.2, rotate: 360 }}
        transition={{ type: 'spring', stiffness: 260, damping: 20 }}
      >
        {number}
      </motion.div>
      <div>
        <h3
          className='text-2xl font-semibold mb-2'
          style={{ color: 'var(--brand-light-gray)' }}
        >
          {title}
        </h3>
        <p
          className='text-lg leading-relaxed'
          style={{ color: 'var(--text-secondary)' }}
        >
          {description}
        </p>
      </div>
    </motion.div>
  )
}

function TestimonialCard({ quote, author, delay }: any) {
  return (
    <motion.div
      className='p-6 rounded-xl'
      style={{
        background: 'var(--brand-dark-gray)',
        border: '1px solid rgba(240, 185, 11, 0.1)',
      }}
      variants={fadeInScale}
      transition={{ delay }}
      whileHover={{
        scale: 1.05,
        borderColor: 'var(--brand-yellow)',
        boxShadow: '0 20px 40px rgba(0, 0, 0, 0.4)',
      }}
    >
      <p className='text-lg mb-4' style={{ color: 'var(--brand-light-gray)' }}>
        "{quote}"
      </p>
      <div className='flex items-center gap-2'>
        <motion.div
          className='w-8 h-8 rounded-full'
          style={{ background: 'var(--binance-yellow)' }}
          whileHover={{ rotate: 180 }}
        />
        <span
          className='text-sm font-semibold'
          style={{ color: 'var(--text-secondary)' }}
        >
          {author}
        </span>
      </div>
    </motion.div>
  )
}

function LoginModal({ onClose }: { onClose: () => void }) {
  return (
    <motion.div
      className='fixed inset-0 z-50 flex items-center justify-center p-4'
      style={{ background: 'rgba(0, 0, 0, 0.8)' }}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className='relative max-w-md w-full rounded-2xl p-8'
        style={{
          background: 'var(--brand-dark-gray)',
          border: '1px solid rgba(240, 185, 11, 0.2)',
        }}
        initial={{ scale: 0.9, y: 50 }}
        animate={{ scale: 1, y: 0 }}
        exit={{ scale: 0.9, y: 50 }}
        onClick={(e) => e.stopPropagation()}
      >
        <motion.button
          onClick={onClose}
          className='absolute top-4 right-4'
          style={{ color: 'var(--text-secondary)' }}
          whileHover={{ scale: 1.1, rotate: 90 }}
          whileTap={{ scale: 0.9 }}
        >
          <X className='w-6 h-6' />
        </motion.button>
        <h2
          className='text-2xl font-bold mb-6'
          style={{ color: 'var(--brand-light-gray)' }}
        >
          访问 NOFX 平台
        </h2>
        <p className='text-sm mb-6' style={{ color: 'var(--text-secondary)' }}>
          请选择登录或注册以访问完整的 AI 交易平台
        </p>
        <div className='space-y-3'>
          <motion.button
            onClick={() => {
              window.history.pushState({}, '', '/login')
              window.dispatchEvent(new PopStateEvent('popstate'))
              onClose()
            }}
            className='block w-full px-6 py-3 rounded-lg font-semibold text-center'
            style={{
              background: 'var(--brand-yellow)',
              color: 'var(--brand-black)',
            }}
            whileHover={{
              scale: 1.05,
              boxShadow: '0 10px 30px rgba(240, 185, 11, 0.4)',
            }}
            whileTap={{ scale: 0.95 }}
          >
            登录
          </motion.button>
          <motion.button
            onClick={() => {
              window.history.pushState({}, '', '/register')
              window.dispatchEvent(new PopStateEvent('popstate'))
              onClose()
            }}
            className='block w-full px-6 py-3 rounded-lg font-semibold text-center'
            style={{
              background: 'var(--brand-dark-gray)',
              color: 'var(--brand-light-gray)',
              border: '1px solid rgba(240, 185, 11, 0.2)',
            }}
            whileHover={{ scale: 1.05, borderColor: 'var(--brand-yellow)' }}
            whileTap={{ scale: 0.95 }}
          >
            注册新账号
          </motion.button>
        </div>
      </motion.div>
    </motion.div>
  )
}
