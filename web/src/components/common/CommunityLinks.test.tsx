import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { BRAND_INFO, OFFICIAL_LINKS } from '../../constants/branding'
import { SiteFooter } from './SiteFooter'
import { WhitelistFullPage } from './WhitelistFullPage'
import FooterSection from '../landing/FooterSection'

const communityURL = 'https://t.me/nofx_dev_community'

afterEach(cleanup)

describe('Telegram community links, independent of removed bot integration', () => {
  it('keeps the official public community address', () => {
    expect(OFFICIAL_LINKS.telegram).toBe(communityURL)
    expect(BRAND_INFO.social.tg()).toBe(communityURL)
  })

  it.each([
    ['site footer', <SiteFooter language="en" />],
    ['landing footer', <FooterSection language="en" />],
    ['support page', <WhitelistFullPage />],
  ])('retains a safe join link in the %s', (_name, element) => {
    render(<MemoryRouter>{element}</MemoryRouter>)
    const links = screen.getAllByRole('link', { name: /community/i })
    for (const link of links) {
      expect(link).toHaveAttribute('href', communityURL)
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    }
    expect(screen.queryByText(/bot token|bot setup|botfather/i)).toBeNull()
  })
})
