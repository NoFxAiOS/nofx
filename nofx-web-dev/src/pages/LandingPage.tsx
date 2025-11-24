import { useState } from 'react'
import { motion } from 'framer-motion'
import { ArrowRight } from 'lucide-react'
import HeaderBar from '../components/landing/HeaderBar'
import HeroSection from '../components/landing/HeroSection'
import AboutSection from '../components/landing/AboutSection'
import FeaturesSection from '../components/landing/FeaturesSection'
import HowItWorksSection from '../components/landing/HowItWorksSection'
// import CommunitySection from '../components/landing/CommunitySection'
import AnimatedSection from '../components/landing/AnimatedSection'
import LoginModal from '../components/landing/LoginModal'
import FooterSection from '../components/landing/FooterSection'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

export function LandingPage() {
  const [showLoginModal, setShowLoginModal] = useState(false)
  const { user, logout } = useAuth()
  const { language, setLanguage } = useLanguage()
  const isLoggedIn = !!user

  console.log('LandingPage - user:', user, 'isLoggedIn:', isLoggedIn)
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
          console.log('LandingPage onPageChange called with:', page)
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
        className="min-h-screen px-4 sm:px-6 lg:px-8"
        style={{
          background: 'var(--background)',
          color: 'var(--text-primary)',
        }}
      >
        <HeroSection language={language} />
        <AboutSection language={language} />
        <FeaturesSection language={language} />
        <HowItWorksSection language={language} />
        {/* <CommunitySection /> */}

        {/* CTA */}
        <AnimatedSection backgroundColor="var(--panel-bg)">
          <div className="max-w-4xl mx-auto text-center">
            <motion.h2
              className="text-5xl font-bold mb-6"
              style={{ color: 'var(--text-primary)' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
            >
              {t('readyToDefine', language)}
            </motion.h2>
            <motion.p
              className="text-xl mb-12"
              style={{ color: 'var(--text-secondary)' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: 0.1 }}
            >
              {t('startWithCrypto', language)}
            </motion.p>
            <div className="flex flex-wrap justify-center gap-4">
              <motion.button
                onClick={() => setShowLoginModal(true)}
                className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg"
                style={{
                  background: 'var(--brand-red)',
                  color: '#FFFFFF',
                }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
              >
                {t('getStartedNow', language)}
                <motion.div
                  animate={{ x: [0, 5, 0] }}
                  transition={{ duration: 1.5, repeat: Infinity }}
                >
                  <ArrowRight className="w-5 h-5" />
                </motion.div>
              </motion.button>
              <motion.button
                onClick={() => (window.location.href = '/faq')}
                className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg"
                style={{
                  background: 'transparent',
                  color: 'var(--text-primary)',
                  border: '2px solid var(--brand-red)',
                }}
                whileHover={{
                  scale: 1.05,
                  backgroundColor: 'rgba(229, 0, 18, 0.1)',
                }}
                whileTap={{ scale: 0.95 }}
              >
                {t('viewFAQ', language)}
              </motion.button>
            </div>
          </div>
        </AnimatedSection>

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
