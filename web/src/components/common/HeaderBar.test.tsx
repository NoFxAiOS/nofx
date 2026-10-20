import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import HeaderBar from './HeaderBar'

function Location() {
  return <output data-testid="location">{useLocation().pathname}</output>
}

describe('promotion navigation', () => {
  it('places the authenticated promotion route between factor mining and research on desktop and mobile', () => {
    const onPageChange = vi.fn()
    render(
      <MemoryRouter>
        <HeaderBar
          language="zh"
          isLoggedIn
          currentPage="paper-promotion"
          onPageChange={onPageChange}
        />
        <Location />
      </MemoryRouter>
    )
    const links = screen.getAllByRole('button')
    const promotionIndex = links.findIndex(
      (button) => button.textContent === '晋升报告'
    )
    expect(links[promotionIndex - 1]).toHaveTextContent('因子挖掘')
    expect(links[promotionIndex + 1]).toHaveTextContent('研究记录')
    fireEvent.click(links[promotionIndex])
    expect(onPageChange).toHaveBeenCalledWith('paper-promotion')
    expect(screen.getByTestId('location')).toHaveTextContent('/paper-promotion')

    fireEvent.click(screen.getByRole('button', { name: 'Toggle menu' }))
    const mobileNav = screen.getAllByRole('navigation')[1]
    const mobileLink = within(mobileNav).getByRole('button', {
      name: '晋升报告',
    })
    expect(mobileLink).toHaveClass('bg-surface-2')
    fireEvent.click(mobileLink)
    expect(onPageChange).toHaveBeenCalledTimes(2)
  })

  it('requires login before opening the promotion page', () => {
    const onLoginRequired = vi.fn()
    render(
      <MemoryRouter>
        <HeaderBar language="en" onLoginRequired={onLoginRequired} />
        <Location />
      </MemoryRouter>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Paper Promotion' }))
    expect(onLoginRequired).toHaveBeenCalledWith('Paper Promotion')
    expect(screen.getByTestId('location')).toHaveTextContent(/^\/$/)
  })
})
