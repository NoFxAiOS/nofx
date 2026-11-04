import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AuthProvider, useAuth } from './AuthContext'

const mocks = vi.hoisted(() => ({ error: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: mocks.error } }))
vi.mock('./LanguageContext', () => ({
  useLanguage: () => ({ language: 'en' }),
}))
vi.mock('../lib/config', () => ({
  getSystemConfig: vi.fn().mockResolvedValue({}),
  invalidateSystemConfig: vi.fn(),
}))
vi.mock('../lib/httpClient', () => ({ reset401Flag: vi.fn(), httpClient: {} }))

function Session() {
  const { user, logout, isLoading } = useAuth()
  return (
    <>
      <p>{isLoading ? 'loading' : user?.email || 'signed out'}</p>
      <button onClick={logout}>Sign out</button>
    </>
  )
}

describe('durable logout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.setItem('auth_token', 'isolated-test-token')
    localStorage.setItem(
      'auth_user',
      JSON.stringify({ id: 'owner', email: 'owner@example.test' })
    )
  })
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  async function logout() {
    render(
      <MemoryRouter>
        <AuthProvider>
          <Session />
        </AuthProvider>
      </MemoryRouter>
    )
    await screen.findByText('owner@example.test')
    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  }

  for (const status of [200, 401]) {
    it(`clears the local session when the server confirms revocation or invalidity (${status})`, async () => {
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({ ok: status === 200, status })
      )
      await logout()
      await screen.findByText('signed out')
      expect(localStorage.getItem('auth_token')).toBeNull()
    })
  }

  for (const networkError of [false, true]) {
    it(`does not pretend logout succeeded on ${networkError ? 'network' : 'database'} failure`, async () => {
      const fetcher = vi.fn()
      if (networkError) fetcher.mockRejectedValue(new Error('offline'))
      else fetcher.mockResolvedValue({ ok: false, status: 503 })
      vi.stubGlobal('fetch', fetcher)
      await logout()
      await waitFor(() => expect(mocks.error).toHaveBeenCalled())
      expect(localStorage.getItem('auth_token')).toBe('isolated-test-token')
      expect(screen.getByText('owner@example.test')).toBeTruthy()
    })
  }
})
