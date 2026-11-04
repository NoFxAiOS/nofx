import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SettingsPage } from './SettingsPage'

const mocks = vi.hoisted(() => ({
  logout: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}))
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { email: 'owner@example.test' },
    logout: mocks.logout,
  }),
}))
vi.mock('../contexts/LanguageContext', () => ({
  useLanguage: () => ({ language: 'en' }),
}))
vi.mock('sonner', () => ({
  toast: { success: mocks.success, error: mocks.error },
}))
vi.mock('../lib/api', () => ({ api: {} }))
vi.mock('../components/trader/ExchangeConfigModal', () => ({
  ExchangeConfigModal: () => null,
}))
vi.mock('../components/trader/ModelConfigModal', () => ({
  ModelConfigModal: () => null,
}))

describe('password change session handling', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.setItem('auth_token', 'test-token')
  })
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  function submit() {
    render(
      <MemoryRouter>
        <SettingsPage />
      </MemoryRouter>
    )
    fireEvent.change(screen.getByPlaceholderText('At least 8 characters'), {
      target: { value: 'new-password' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Update Password' }))
  }

  it('signs out after a successful change and explains why', async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true })
    vi.stubGlobal('fetch', fetcher)
    submit()
    await waitFor(() => expect(mocks.logout).toHaveBeenCalledOnce())
    expect(mocks.success).toHaveBeenCalledWith(
      expect.stringContaining('all previous sessions have expired')
    )
    expect(fetcher).toHaveBeenCalledWith(
      '/api/user/password',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ new_password: 'new-password' }),
      })
    )
  })

  it('does not sign out or claim success if the password update fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        json: async () => ({ error: 'Update failed' }),
      })
    )
    submit()
    await waitFor(() =>
      expect(mocks.error).toHaveBeenCalledWith('Update failed')
    )
    expect(mocks.logout).not.toHaveBeenCalled()
    expect(mocks.success).not.toHaveBeenCalled()
  })
})
