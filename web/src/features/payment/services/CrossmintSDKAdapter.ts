/**
 * Crossmint SDK Adapter
 * Adapts official Crossmint SDK to our service interface
 *
 * Design Patterns:
 * - Adapter Pattern: Wraps SDK to match our interface
 * - Single Responsibility: Only handles SDK integration
 * - Dependency Inversion: Depends on abstraction (interface)
 *
 * Benefits:
 * - Decouples application from SDK specifics
 * - Easy to test (can mock SDK)
 * - Easy to swap implementations
 */

import type { ICrossmintService, CheckoutConfig } from "./ICrossmintService"
import type { PaymentPackage, CrossmintLineItem } from "../types/payment"

/**
 * SDK Adapter Implementation
 * Uses official @crossmint/client-sdk-react-ui
 */
export class CrossmintSDKAdapter implements ICrossmintService {
  private apiKey: string
  private environment: "staging" | "production"

  constructor(apiKey?: string, environment: "staging" | "production" = "staging") {
    this.apiKey = apiKey || import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY || ""
    this.environment = environment

    if (!this.apiKey) {
      console.warn(
        "[Crossmint SDK] API Key not configured. Payment feature will not work."
      )
    }
  }

  /**
   * Checks if SDK is properly configured
   */
  isConfigured(): boolean {
    return !!this.apiKey && this.apiKey.length > 0
  }

  /**
   * Initialize checkout using SDK
   * Note: SDK handles the API calls internally
   */
  async initializeCheckout(config: CheckoutConfig): Promise<string> {
    if (!this.isConfigured()) {
      throw new Error("Crossmint API Key is not configured")
    }

    try {
      // Import SDK dynamically to avoid SSR issues
      const { CrossmintCheckoutService } = await import(
        "@crossmint/client-sdk-react-ui"
      )

      // Initialize SDK service
      const checkoutService = CrossmintCheckoutService.init({
        clientApiKey: this.apiKey,
        environment: this.environment,
      })

      // Create checkout order
      const order = await checkoutService.createOrder({
        lineItems: this.transformLineItems(config.lineItems),
        payment: {
          method: config.checkoutProps?.payment?.allowedMethods?.[0] || "crypto",
          currency: config.lineItems[0]?.currency || "USDT",
        },
        locale: config.locale || "en-US",
      })

      if (!order || !order.orderId) {
        throw new Error("No order ID returned from Crossmint SDK")
      }

      console.log("[Crossmint SDK] Order created:", order.orderId)
      return order.orderId
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown error"
      console.error("[Crossmint SDK] Failed to initialize checkout:", message)
      throw new Error(`Failed to initialize Crossmint checkout: ${message}`)
    }
  }

  /**
   * Transform our line items to SDK format
   * Adapts between domain model and SDK model
   */
  private transformLineItems(items: CrossmintLineItem[]): any[] {
    return items.map((item) => ({
      price: item.price,
      currency: item.currency,
      quantity: item.quantity || 1,
      metadata: item.metadata || {},
    }))
  }

  /**
   * Creates line items from payment package
   * Maintains compatibility with existing code
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
   * Gets SDK configuration
   */
  getConfig(): { apiKey: string; configured: boolean } {
    return {
      apiKey: this.apiKey,
      configured: this.isConfigured(),
    }
  }

  /**
   * Resets adapter state
   * Note: SDK is stateless, so this is a no-op
   */
  reset(): void {
    // SDK doesn't maintain state, nothing to reset
    console.log("[Crossmint SDK] Reset called (no-op)")
  }
}
