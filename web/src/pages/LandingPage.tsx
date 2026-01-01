import { useState } from 'react'
import HeaderBar from '../components/HeaderBar'
import LoginModal from '../components/landing/LoginModal'
import FooterSection from '../components/landing/FooterSection'
import TerminalHero from '../components/landing/core/TerminalHero'
import LiveFeed from '../components/landing/core/LiveFeed'
import AgentGrid from '../components/landing/core/AgentGrid'
import DeploymentHub from '../components/landing/core/DeploymentHub'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'

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
      <div className="min-h-screen bg-nofx-bg text-nofx-text font-sans selection:bg-nofx-gold selection:text-black">

        <TerminalHero />

        <LiveFeed />

        <AgentGrid />

        <DeploymentHub />

        <FooterSection language={language} />

        {showLoginModal && (
          <LoginModal
            onClose={() => setShowLoginModal(false)}
            language={language}
          />
        )}
      </div>
    </>
  )
}
