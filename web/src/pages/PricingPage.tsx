/**
 * Pricing Page
 *
 * Displays all available credit packages for purchase
 * Integrates with PaymentModal for checkout
 */

import React, { useState, useMemo } from 'react'
import PricingCard from '@/features/payment/components/PricingCard'
import usePricingData from '@/features/payment/hooks/usePricingData'
import PaymentModal from '@/features/payment/components/PaymentModal'
import { getPricingContent } from '@/features/payment/constants/pricing-content'
import '@/features/payment/styles/pricing.css'
import type { PaymentPackage } from '@/features/payment/types/payment'

interface PricingPageProps {
  lang?: 'en' | 'zh'
}

const PricingPage: React.FC<PricingPageProps> = ({ lang = 'en' }) => {
  const { packages, loading, error, refetch } = usePricingData()
  const [selectedPackage, setSelectedPackage] = useState<PaymentPackage | null>(
    null
  )
  const [showPaymentModal, setShowPaymentModal] = useState(false)

  const content = useMemo(() => getPricingContent(lang), [lang])

  const handlePurchase = (packageId: string) => {
    const pkg = packages.find((p) => p.id === packageId)
    if (pkg) {
      setSelectedPackage(pkg)
      setShowPaymentModal(true)
    }
  }

  const handleCloseModal = () => {
    setShowPaymentModal(false)
    setSelectedPackage(null)
  }

  return (
    <div className="pricing-page">
      {/* Header */}
      <section className="pricing-header">
        <div className="pricing-header-content">
          <h1 className="pricing-title">
            {lang === 'zh' ? '积分套餐' : 'Credit Packages'}
          </h1>
          <p className="pricing-subtitle">
            {lang === 'zh'
              ? '选择最适合您的套餐，开始交易'
              : 'Choose the perfect plan for your trading needs'}
          </p>
        </div>
      </section>

      {/* Pricing Cards Grid */}
      <section className="pricing-cards-section">
        {error && !packages.length ? (
          <div className="pricing-error">
            <p>
              {lang === 'zh'
                ? '加载套餐失败。正在使用默认套餐...'
                : 'Failed to load packages. Using default packages...'}
            </p>
            <button
              className="pricing-retry-button"
              onClick={() => refetch()}
            >
              {lang === 'zh' ? '重试' : 'Retry'}
            </button>
          </div>
        ) : null}

        {loading && !packages.length ? (
          <div className="pricing-skeleton">
            {[1, 2, 3].map((i) => (
              <div key={i} className="skeleton-card" />
            ))}
          </div>
        ) : (
          <div className="pricing-grid">
            {packages.map((pkg) => (
              <PricingCard
                key={pkg.id}
                package={pkg}
                onPurchase={handlePurchase}
                isDisabled={pkg.isActive === false}
              />
            ))}
          </div>
        )}
      </section>

      {/* Features */}
      <section className="pricing-features-section">
        <div className="features-container">
          <h2 className="features-title">
            {lang === 'zh'
              ? '所有套餐都包含以下功能'
              : 'All packages include'}
          </h2>
          <div className="features-grid">
            {content.features.map((feature, idx) => (
              <div key={idx} className="feature-item">
                <span className="feature-icon">✓</span>
                <span className="feature-text">{feature}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="pricing-faq-section">
        <div className="faq-container">
          <h2 className="faq-title">
            {lang === 'zh' ? '常见问题' : 'Frequently Asked Questions'}
          </h2>
          <div className="faq-list">
            {content.faq.map((item, idx) => (
              <details key={idx} className="faq-item">
                <summary className="faq-question">
                  {item.question}
                </summary>
                <p className="faq-answer">{item.answer}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      {/* Payment Modal */}
      {showPaymentModal && selectedPackage && (
        <PaymentModal
          package={selectedPackage}
          onClose={handleCloseModal}
        />
      )}
    </div>
  )
}

export default PricingPage
