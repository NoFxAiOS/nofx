import { useState } from 'react'
import HeaderBar from '../components/common/HeaderBar'
import LoginModal from '../components/landing/LoginModal'
import { LoginRequiredOverlay } from '../components/auth/LoginRequiredOverlay'
import FooterSection from '../components/landing/FooterSection'
import Hero from '../components/landing/Hero'
import DeploymentHub from '../components/landing/core/DeploymentHub'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'

export function LandingPage() {
  const [showLoginModal, setShowLoginModal] = useState(false)
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')
  const { user, logout } = useAuth()
  const { language, setLanguage } = useLanguage()
  const isLoggedIn = !!user

  const handleLoginRequired = (featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }

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
        onLoginRequired={handleLoginRequired}
        onPageChange={(page) => {
          const pathMap: Record<string, string> = {
            data: '/data',
            competition: '/competition',
            'strategy-market': '/strategy-market',
            traders: '/traders',
            trader: '/dashboard',
            strategy: '/strategy',
          }
          const path = pathMap[page]
          if (path) {
            window.location.href = path
          }
        }}
      />
      <div className="min-h-screen bg-bg pt-[52px] text-fg">
        <Hero />

        <DeploymentHub />

        <FooterSection language={language} />

        {showLoginModal && (
          <LoginModal
            onClose={() => setShowLoginModal(false)}
            language={language}
          />
        )}

        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    </>
  )
}
