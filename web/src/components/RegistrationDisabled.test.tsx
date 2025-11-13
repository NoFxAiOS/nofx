import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { RegistrationDisabled } from './RegistrationDisabled'
import { LanguageProvider } from '../contexts/LanguageContext'

// Mock useLanguage hook
vi.mock('../contexts/LanguageContext', async () => {
  const actual = await vi.importActual('../contexts/LanguageContext')
  return {
    ...actual,
    useLanguage: () => ({ language: 'en' }),
  }
})

/**
 * RegistrationDisabled Component Tests
 *
 * Tests the component that displays when registration is disabled
 * This is part of the registration toggle feature
 */
describe('RegistrationDisabled Component', () => {
  const renderComponent = () => {
    return render(
      <LanguageProvider>
        <RegistrationDisabled />
      </LanguageProvider>
    )
  }

  describe('Rendering', () => {
    it('should render the component without errors', () => {
      const { container } = renderComponent()
      expect(container).toBeTruthy()
    })

    it('should display the NoFx logo', () => {
      renderComponent()
      const logo = screen.getByAltText('NoFx Logo')
      expect(logo).toBeTruthy()
      expect(logo.getAttribute('src')).toBe('/icons/nofx.svg')
    })

    it('should display registration closed heading', () => {
      renderComponent()
      const heading = screen.getByText('Registration Closed')
      expect(heading).toBeTruthy()
    })

    it('should display registration closed message', () => {
      renderComponent()
      const message = screen.getByText(/User registration is currently disabled/i)
      expect(message).toBeTruthy()
    })

    it('should display back to login button', () => {
      renderComponent()
      const button = screen.getByRole('button', { name: /back to login/i })
      expect(button).toBeTruthy()
    })
  })

  describe('Navigation', () => {
    it('should navigate to login page when button is clicked', () => {
      const pushStateSpy = vi.spyOn(window.history, 'pushState')
      const dispatchEventSpy = vi.spyOn(window, 'dispatchEvent')

      renderComponent()
      const button = screen.getByRole('button', { name: /back to login/i })

      fireEvent.click(button)

      expect(pushStateSpy).toHaveBeenCalledWith({}, '', '/login')
      expect(dispatchEventSpy).toHaveBeenCalled()

      pushStateSpy.mockRestore()
      dispatchEventSpy.mockRestore()
    })
  })

  describe('Styling', () => {
    it('should have correct background color', () => {
      const { container } = renderComponent()
      const mainDiv = container.firstChild as HTMLElement
      // Browser converts hex to rgb
      expect(mainDiv.style.background).toMatch(/rgb\(11,\s*14,\s*17\)|#0B0E11/i)
    })

    it('should have correct text color', () => {
      const { container } = renderComponent()
      const mainDiv = container.firstChild as HTMLElement
      // Browser converts hex to rgb
      expect(mainDiv.style.color).toMatch(/rgb\(234,\s*236,\s*239\)|#EAECEF/i)
    })

    it('should have centered layout', () => {
      const { container } = renderComponent()
      const mainDiv = container.firstChild as HTMLElement
      expect(mainDiv.className).toContain('flex')
      expect(mainDiv.className).toContain('items-center')
      expect(mainDiv.className).toContain('justify-center')
    })
  })
})
