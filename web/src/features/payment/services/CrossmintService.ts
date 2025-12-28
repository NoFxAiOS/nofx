/**
 * Crossmint Service
 * Wrapper around Crossmint SDK
 * Handles all interactions with Crossmint API
 */

import type { PaymentPackage, CrossmintLineItem } from "../types/payment"

export class CrossmintService {
  private apiKey: string

  constructor(apiKey?: string) {
    this.apiKey = apiKey || import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY || ""

    if (!this.apiKey) {
      console.warn(
        "[Crossmint] API Key not configured. Payment feature will not work."
      )
    }
  }

  /**
   * Checks if Crossmint SDK is properly configured
   */
  isConfigured(): boolean {
    return !!this.apiKey
  }

  /**
   * Initialize Crossmint checkout
   * This is typically called by React components
   */
  async initializeCheckout(): Promise<void> {
    if (!this.isConfigured()) {
      throw new Error("Crossmint API Key is not configured")
    }

    // The actual SDK initialization is handled by CrossmintProvider
    // This method is for reference and future enhancements
  }

  /**
   * Creates line items in Crossmint format
   */
  createLineItems(pkg: PaymentPackage): CrossmintLineItem[] {
    const totalCredits = pkg.credits.amount + (pkg.credits.bonusAmount || 0)

    return [
      {
        price: pkg.price.amount.toString(),
        currency: pkg.price.currency,
        quantity: 1,
        metadata: {
          packageId: pkg.id,
          credits: totalCredits,
          bonusMultiplier: pkg.credits.bonusMultiplier || 1.0,
        },
      },
    ]
  }

  /**
   * Handles Crossmint checkout events
   */
  handleCheckoutEvent(event: any): {
    type: string
    handled: boolean
    message?: string
  } {
    if (!event || !event.type) {
      return {
        type: "unknown",
        handled: false,
        message: "Invalid event",
      }
    }

    switch (event.type) {
      case "checkout:order.created":
        return {
          type: "order.created",
          handled: true,
          message: "Order created",
        }

      case "checkout:order.paid":
        return {
          type: "order.paid",
          handled: true,
          message: "Payment confirmed on blockchain",
        }

      case "checkout:order.failed":
        return {
          type: "order.failed",
          handled: true,
          message: "Payment failed",
        }

      case "checkout:order.cancelled":
        return {
          type: "order.cancelled",
          handled: true,
          message: "User cancelled payment",
        }

      default:
        return {
          type: event.type,
          handled: false,
          message: `Unknown event type: ${event.type}`,
        }
    }
  }

  /**
   * Verifies a Crossmint payment signature
   * For enhanced security (optional but recommended)
   */
  verifyPaymentSignature(signature: unknown): boolean {
    // In a real implementation, this would verify the signature
    // using the Crossmint webhook secret
    // For now, we accept it and let the backend verify

    if (!signature || typeof signature !== "string") {
      return false
    }

    // Basic check - in production, use HMAC verification
    return signature.length > 0
  }

  /**
   * Gets Crossmint SDK configuration
   */
  getConfig(): { apiKey: string; configured: boolean } {
    return {
      apiKey: this.apiKey,
      configured: this.isConfigured(),
    }
  }

  /**
   * Resets the service state
   */
  reset(): void {
    // Service state reset (currently no state to reset)
  }
}

// Export singleton instance
export const crossmintService = new CrossmintService()
