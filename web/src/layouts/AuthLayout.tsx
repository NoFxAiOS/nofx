import { ReactNode } from 'react'
import { Outlet, Link } from 'react-router-dom'
import { Container } from '../components/Container'

interface AuthLayoutProps {
  children?: ReactNode
}

export default function AuthLayout({ children }: AuthLayoutProps) {
  return (
    <div className="min-h-screen" style={{ background: 'var(--background)' }}>
      {/* Simple Header with Logo */}
      <nav
        className="fixed top-0 w-full z-50"
        style={{
          background: 'var(--header-bg)',
          borderBottom: '1px solid var(--panel-border)',
          backdropFilter: 'blur(10px)',
        }}
      >
        <Container className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link
            to="/"
            className="flex items-center gap-3 hover:opacity-80 transition-opacity"
          >
            <img src="/icons/ares.svg" alt="Ares Logo" className="w-8 h-8" />
            <span className="text-xl font-bold" style={{ color: 'var(--accent-red)' }}>
              Ares
            </span>
          </Link>
        </Container>
      </nav>

      {/* Content with top padding to avoid overlap with fixed header */}
      <div className="pt-16">{children || <Outlet />}</div>
    </div>
  )
}
