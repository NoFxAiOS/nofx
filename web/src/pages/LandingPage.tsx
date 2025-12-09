import { useState } from 'react'
import { motion } from 'framer-motion'
import { ArrowRight, Github } from 'lucide-react'
import HeaderBar from '../components/HeaderBar'
import HeroSection from '../components/landing/HeroSection'
import AboutSection from '../components/landing/AboutSection'
import FeaturesSection from '../components/landing/FeaturesSection'
import HowItWorksSection from '../components/landing/HowItWorksSection'
import CommunitySection from '../components/landing/CommunitySection'
import LoginModal from '../components/landing/LoginModal'
import FooterSection from '../components/landing/FooterSection'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { OFFICIAL_LINKS } from '../constants/branding'

export function LandingPage() {
  const [showLoginModal, setShowLoginModal] = useState(false)
  const { user, logout } = useAuth()
  const { language, setLanguage } = useLanguage()
  const isLoggedIn = !!user

  return (
    <>
      <HeaderBar
        onLoginClick={() => setShowLoginModal(true)}
        isLoggedIn={isLoggedIn}
        isHomePage={true}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          if (page === 'competition') {
            window.location.href = '/competition'
          } else if (page === 'traders') {
            window.location.href = '/traders'
          } else if (page === 'trader') {
            window.location.href = '/dashboard'
          }
        }}
      />
      <div
        className="min-h-screen"
        style={{
          background: '#0B0E11',
          color: '#EAECEF',
        }}
      >
        <HeroSection language={language} />
        <AboutSection language={language} />
        <FeaturesSection language={language} />
        <HowItWorksSection language={language} />
        <CommunitySection language={language} />

        {/* Final CTA Section */}
        <section className="py-24 relative overflow-hidden" style={{ background: '#0D1117' }}>
          {/* Background Glow */}
          <div
            className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full blur-3xl opacity-30"
            style={{ background: 'radial-gradient(circle, rgba(240, 185, 11, 0.15) 0%, transparent 70%)' }}
          />

          <div className="max-w-4xl mx-auto px-4 text-center relative z-10">
            <motion.h2
              className="text-4xl lg:text-5xl font-bold mb-6"
              style={{ color: '#EAECEF' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
            >
              {t('readyToDefine', language)}
            </motion.h2>
            <motion.p
              className="text-lg mb-10 max-w-2xl mx-auto"
              style={{ color: '#848E9C' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: 0.1 }}
            >
              {t('startWithCrypto', language)}
            </motion.p>

            <motion.div
              className="flex flex-col sm:flex-row items-center justify-center gap-4"
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: 0.2 }}
            >
              <motion.button
                onClick={() => setShowLoginModal(true)}
                className="group flex items-center gap-3 px-8 py-4 rounded-xl font-bold text-lg"
                style={{
                  background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                  color: '#0B0E11',
                  boxShadow: '0 4px 24px rgba(240, 185, 11, 0.3)',
                }}
                whileHover={{
                  scale: 1.02,
                  boxShadow: '0 8px 32px rgba(240, 185, 11, 0.4)',
                }}
                whileTap={{ scale: 0.98 }}
              >
                {t('getStartedNow', language)}
                <ArrowRight className="w-5 h-5 transition-transform group-hover:translate-x-1" />
              </motion.button>

              <motion.a
                href={OFFICIAL_LINKS.github}
                target="_blank"
                rel="noopener noreferrer"
                className="group flex items-center gap-3 px-8 py-4 rounded-xl font-bold text-lg"
                style={{
                  background: 'rgba(255, 255, 255, 0.05)',
                  color: '#EAECEF',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                }}
                whileHover={{
                  scale: 1.02,
                  background: 'rgba(255, 255, 255, 0.08)',
                  borderColor: 'rgba(240, 185, 11, 0.3)',
                }}
                whileTap={{ scale: 0.98 }}
              >
                <Github className="w-5 h-5" />
                {t('viewSourceCode', language)}
              </motion.a>
            </motion.div>
          </div>
        </section>

        {showLoginModal && (
          <LoginModal
            onClose={() => setShowLoginModal(false)}
            language={language}
          />
        )}
        <FooterSection language={language} />
      </div>
    </>
  )
}
