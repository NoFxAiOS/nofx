/**
 * Crossmint Checkout Embed Component
 * Wraps official Crossmint SDK component
 *
 * Design Principle: KISS (Keep It Simple, Stupid)
 * - Single responsibility: Display Crossmint embedded checkout
 * - Minimal props: Only what's needed
 * - Clean interface: Easy to use and test
 */

import { CrossmintEmbeddedCheckout } from "@crossmint/client-sdk-react-ui"
import type { PaymentPackage } from "../types/payment"

interface CrossmintCheckoutEmbedProps {
  /** Payment package to purchase */
  package: PaymentPackage
  /** Callback when payment succeeds */
  onSuccess: (orderId: string) => void
  /** Callback when payment fails */
  onError: (error: string) => void
  /** API Key for Crossmint */
  apiKey: string
}

/**
 * Crossmint Embedded Checkout Component
 * Uses official SDK - no custom API calls needed
 */
export function CrossmintCheckoutEmbed({
  package: pkg,
  onSuccess,
  onError,
  apiKey,
}: CrossmintCheckoutEmbedProps) {
  // Transform package to Crossmint line items format
  const totalCredits = pkg.credits.amount + (pkg.credits.bonusAmount || 0)

  return (
    <CrossmintEmbeddedCheckout
      // Line items define what user is purchasing
      lineItems={{
        collectionLocator: `crossmint:${pkg.id}`,
        callData: {
          totalPrice: pkg.price.amount.toString(),
          quantity: 1,
        },
      }}
      // Payment configuration
      payment={{
        crypto: {
          enabled: true,
          defaultCurrency: pkg.price.currency.toLowerCase(),
        },
      }}
      // Locale
      locale="en-US"
      // Event handlers
      onEvent={(event) => {
        console.log("[Crossmint Event]", event.type, event.payload)

        switch (event.type) {
          case "order:process.finished":
            // Payment completed successfully
            if (event.payload.successfulTransactionIdentifiers?.length > 0) {
              const orderId = event.payload.successfulTransactionIdentifiers[0]
              onSuccess(orderId)
            }
            break

          case "payment:process.rejected":
          case "payment:preparation.failed":
          case "transaction:fulfillment.failed":
            // Payment failed
            onError(event.payload.error?.message || "Payment failed")
            break

          case "payment:process.canceled":
            // User cancelled
            onError("Payment cancelled by user")
            break

          default:
            // Other events - just log
            break
        }
      }}
    />
  )
}
