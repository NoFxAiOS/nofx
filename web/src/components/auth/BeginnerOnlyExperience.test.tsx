import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HeaderBar from '../common/HeaderBar'
import { SetupPage } from '../modals/SetupPage'

const mocks = vi.hoisted(() => ({
  register: vi.fn(),
}))

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ register: mocks.register }),
}))

vi.mock('../../contexts/LanguageContext', () => ({
  useLanguage: () => ({ language: 'en' }),
}))

vi.mock('../common/HyperliquidWalletConnect', () => ({
  HyperliquidWalletConnect: () => null,
}))

describe('beginner-only live trading experience', () => {
  beforeEach(() => {
    localStorage.clear()
    mocks.register.mockReset()
    mocks.register.mockResolvedValue({ success: true })
  })

  it('does not offer Advanced mode during first-time setup', () => {
    render(
      <MemoryRouter>
        <SetupPage />
      </MemoryRouter>
    )

    expect(screen.queryByText(/Advanced Mode/i)).toBeNull()
    expect(
      screen.queryByText(/configure models, wallets, and exchanges/i)
    ).toBeNull()
  })

  it('always registers the first owner into the guided flow', async () => {
    render(
      <MemoryRouter>
        <SetupPage />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('you@example.com'), {
      target: { value: 'owner@example.com' },
    })
    fireEvent.change(screen.getByPlaceholderText('At least 8 characters'), {
      target: { value: 'StrongPass123!' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Get Started' }))

    await waitFor(() => {
      expect(mocks.register).toHaveBeenCalledWith(
        'owner@example.com',
        'StrongPass123!',
        undefined,
        'beginner'
      )
    })
  })

  it('does not expose a mode switch after login', () => {
    render(
      <MemoryRouter initialEntries={['/traders']}>
        <HeaderBar
          isLoggedIn
          user={{ email: 'owner@example.com' }}
          onLogout={() => undefined}
        />
      </MemoryRouter>
    )

    fireEvent.click(screen.getByRole('button', { name: /owner@example.com/i }))

    expect(screen.queryByText(/Switch to Advanced/i)).toBeNull()
    expect(screen.queryByText(/Switch to Beginner/i)).toBeNull()
  })

  it('keeps desktop navigation labels on one line', () => {
    render(
      <MemoryRouter initialEntries={['/traders']}>
        <HeaderBar />
      </MemoryRouter>
    )

    expect(screen.getByRole('button', { name: 'Config' }).className).toContain(
      'whitespace-nowrap'
    )
  })

  it('keeps the full navigation visible in the desktop header', () => {
    render(
      <MemoryRouter initialEntries={['/traders']}>
        <HeaderBar />
      </MemoryRouter>
    )

    expect(
      screen.getByRole('button', { name: 'Config' }).parentElement?.className
    ).toContain('lg:flex')
    expect(
      screen.getByRole('button', { name: 'Open navigation' }).className
    ).toContain('lg:hidden')
  })
})
