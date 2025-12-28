/**
 * Payment Modal Component
 * Main container for payment feature
 * Displays package selection and Crossmint checkout with full accessibility support
 */

import { useEffect, useRef, useCallback } from 'react'
import { usePaymentContext } from "../contexts/PaymentProvider"
import { usePaymentPackages } from "../hooks/usePaymentPackages"
import { formatPrice, formatCredits } from "../utils/formatPrice"
import type { PaymentPackage } from "../types/payment"
import styles from "../styles/payment-modal.module.css"

interface PaymentModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: (creditsAdded: number) => void
}

export function PaymentModal({
  isOpen,
  onClose,
  onSuccess,
}: PaymentModalProps) {
  const context = usePaymentContext()
  const { packages } = usePaymentPackages()
  const contentRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLElement | null>(null)

  const handleClose = useCallback(() => {
    context.resetPayment()
    onClose()
    // Restore focus to trigger element
    setTimeout(() => {
      triggerRef.current?.focus()
    }, 0)
  }, [context, onClose])

  // Handle Escape key to close modal
  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, handleClose])

  // Focus management: store trigger element and restore focus on close
  useEffect(() => {
    if (isOpen) {
      triggerRef.current = document.activeElement as HTMLElement
      // Move focus to close button
      const closeButton = contentRef.current?.querySelector('button[aria-label="Close payment modal"]')
      if (closeButton instanceof HTMLElement) {
        closeButton.focus()
      }
    }
  }, [isOpen])

  const handlePackageSelect = (pkg: PaymentPackage) => {
    context.selectPackage(pkg.id)
  }

  const handlePaymentSuccess = () => {
    if (onSuccess) {
      onSuccess(context.creditsAdded)
    }
    context.resetPayment()
    handleClose()
  }

  if (!isOpen) return null

  const apiKey = import.meta.env.VITE_CROSSMINT_CLIENT_API_KEY

  if (!apiKey) {
    return (
      <div className={styles.overlay} role="presentation">
        <div
          className={styles.content}
          role="dialog"
          aria-label="Configuration error"
          aria-modal="true"
        >
          <h2 className={styles.title}>⚠️ 支付功能暂时不可用</h2>
          <p className={styles.description}>请联系管理员配置支付系统</p>
          <button
            className={styles.closeButton}
            onClick={handleClose}
            aria-label="Close configuration error dialog"
          >
            关闭
          </button>
        </div>
      </div>
    )
  }

  return (
    <div
      className={styles.overlay}
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          handleClose()
        }
      }}
    >
      <div
        ref={contentRef}
        className={styles.content}
        role="dialog"
        aria-labelledby="modal-title"
        aria-modal="true"
      >
        {/* Header */}
        <div className={styles.header}>
          <h2 id="modal-title" className={styles.title}>充值积分</h2>
          <button
            onClick={handleClose}
            className={styles.closeButton}
            aria-label="Close payment modal"
            title="Press Escape to close (Esc)"
          >
            ✕
          </button>
        </div>

        {/* Idle State - Package Selection */}
        {context.paymentStatus === "idle" && (
          <div className={styles.idleSection}>
            <p className={styles.description}>
              选择你想要购买的积分套餐
            </p>
            <div
              className={styles.packageGrid}
              role="group"
              aria-label="Payment packages"
            >
              {packages.map(pkg => (
                <button
                  key={pkg.id}
                  onClick={() => handlePackageSelect(pkg)}
                  className={`${styles.packageButton} ${
                    context.selectedPackage?.id === pkg.id ? styles.selected : ''
                  }`}
                  aria-pressed={context.selectedPackage?.id === pkg.id}
                  aria-label={`${pkg.name} - ${formatPrice(pkg.price.amount)} - ${formatCredits(pkg.credits.amount + (pkg.credits.bonusAmount || 0))} credits`}
                >
                  <h4 className={styles.packageName}>{pkg.name}</h4>
                  <p className={styles.packagePrice}>
                    {formatPrice(pkg.price.amount)}
                  </p>
                  <p className={styles.packageCredits}>
                    {formatCredits(
                      pkg.credits.amount + (pkg.credits.bonusAmount || 0)
                    )}{" "}
                    积分
                  </p>
                  {pkg.badge && (
                    <span className={styles.packageBadge}>
                      {pkg.badge}
                    </span>
                  )}
                </button>
              ))}
            </div>

            <button
              onClick={async () => {
                if (context.selectedPackage) {
                  await context.initiatePayment(context.selectedPackage.id)
                }
              }}
              disabled={!context.selectedPackage}
              className={styles.payButton}
              aria-busy={false}
            >
              继续支付
            </button>
          </div>
        )}

        {/* Loading State */}
        {context.paymentStatus === "loading" && (
          <div className={styles.loadingContainer} role="status" aria-live="polite" aria-label="Payment processing">
            <div className={styles.spinner} aria-hidden="true" />
            <p className={styles.loadingText}>处理中...</p>
          </div>
        )}

        {/* Success State */}
        {context.paymentStatus === "success" && (
          <div className={styles.successContainer} role="status" aria-live="polite">
            <div className={styles.successIcon} aria-hidden="true">✓</div>
            <h3 className={styles.successTitle}>支付成功！</h3>
            <p className={styles.successMessage}>
              已获得{" "}
              <span className={styles.successHighlight}>
                {formatCredits(context.creditsAdded)}
              </span>{" "}
              积分
            </p>
            <button
              onClick={handlePaymentSuccess}
              className={styles.completeButton}
              aria-label="Complete payment and close modal"
            >
              完成
            </button>
          </div>
        )}

        {/* Error State */}
        {context.paymentStatus === "error" && (
          <div className={styles.errorContainer} role="alert" aria-live="assertive">
            <div className={styles.errorIcon} aria-hidden="true">
              ✕
            </div>
            <h3 className={styles.errorTitle}>支付失败</h3>
            <p className={styles.errorMessage}>
              {context.error || "发生错误，请重试"}
            </p>
            <div className={styles.errorButtonGroup}>
              <button
                onClick={() => {
                  context.resetPayment()
                }}
                className={styles.retryButton}
                aria-label="Retry payment"
              >
                重试
              </button>
              <button
                onClick={handleClose}
                className={styles.closeErrorButton}
                aria-label="Close payment modal and cancel"
              >
                关闭
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
