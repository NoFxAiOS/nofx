/**
 * Crossmint Service Interface
 * Defines the contract for Crossmint payment integration
 *
 * Design: Interface Segregation Principle (ISP)
 * - Clean abstraction for payment checkout
 * - Allows multiple implementations (legacy API, SDK, mock)
 */

import type { PaymentPackage, CrossmintLineItem } from "../types/payment"

export interface CheckoutConfig {
  lineItems: CrossmintLineItem[]
  checkoutProps?: {
    payment?: {
      allowedMethods?: string[]
    }
    preferredChains?: string[]
  }
  successCallbackURL?: string
  failureCallbackURL?: string
  locale?: string
}

/**
 * Crossmint Service Interface
 * Implemented by both legacy service and SDK adapter
 */
export interface ICrossmintService {
  /**
   * Checks if service is properly configured with API key
   */
  isConfigured(): boolean

  /**
   * Initializes checkout session and returns session ID
   * @throws Error if configuration invalid or API call fails
   */
  initializeCheckout(config: CheckoutConfig): Promise<string>

  /**
   * Creates line items in Crossmint format from payment package
   */
  createLineItems(pkg: PaymentPackage): CrossmintLineItem[]

  /**
   * Gets current configuration status
   */
  getConfig(): { apiKey: string; configured: boolean }

  /**
   * Resets service state (cleanup)
   */
  reset(): void
}
