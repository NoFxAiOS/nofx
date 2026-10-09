import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { LanguageProvider, useLanguage } from './LanguageContext'
import { Header } from '../components/common/Header'

function LanguageProbe() {
  const { language } = useLanguage()
  return <div>{language}</div>
}

describe('English-only language policy', () => {
  beforeEach(() => localStorage.clear())

  it('ignores stale Chinese browser state and normalizes storage to English', async () => {
    localStorage.setItem('language', 'zh')

    render(
      <LanguageProvider>
        <LanguageProbe />
      </LanguageProvider>
    )

    expect(screen.getByText('en')).toBeTruthy()
    await waitFor(() => expect(localStorage.getItem('language')).toBe('en'))
  })

  it('does not render a language switcher anywhere in the English-only header', () => {
    render(
      <LanguageProvider>
        <Header simple />
      </LanguageProvider>
    )

    expect(screen.queryByRole('button', { name: 'Chinese' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'EN' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'ID' })).toBeNull()
  })
})
