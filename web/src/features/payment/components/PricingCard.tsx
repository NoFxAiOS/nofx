/**
 * PricingCard Component
 *
 * Displays a single credit package with pricing and credits information.
 * Features: responsive design, badge support, bonus credits calculation
 */

import React, { useMemo } from 'react'
import '../styles/pricing.css'
import type { PaymentPackage } from '../types/payment'

interface PricingCardProps {
  package: PaymentPackage
  isRecommended?: boolean
  onPurchase?: (packageId: string) => void
  isDisabled?: boolean
}

export const PricingCard: React.FC<PricingCardProps> = ({
  package: pkg,
  isRecommended = false,
  onPurchase,
  isDisabled = false,
}) => {
  // 计算总积分（基础 + 赠送）
  const totalCredits = useMemo(() => {
    return pkg.credits.amount + (pkg.credits.bonusAmount || 0)
  }, [pkg.credits])

  // 计算成本效益（每 USDT 能获得多少积分）
  const creditPerUsd = useMemo(() => {
    return (totalCredits / pkg.price.amount).toFixed(1)
  }, [totalCredits, pkg.price.amount])

  const handlePurchase = () => {
    if (!isDisabled && onPurchase) {
      onPurchase(pkg.id)
    }
  }

  const borderColor = pkg.highlightColor || '#e5e7eb'
  const bgColor = pkg.highlightColor ? `${pkg.highlightColor}15` : '#ffffff'

  return (
    <div
      className="pricing-card"
      style={{
        borderColor: pkg.badge ? borderColor : '#e5e7eb',
        backgroundColor: bgColor,
      }}
    >
      {/* Header: Badge and Title */}
      <div className="pricing-card-header">
        <h3 className="pricing-card-title">{pkg.name}</h3>
        {pkg.badge && (
          <span className="pricing-card-badge" style={{ borderColor }}>
            {pkg.badge}
          </span>
        )}
      </div>

      {/* Description */}
      {pkg.description && (
        <p className="pricing-card-description">{pkg.description}</p>
      )}

      {/* Price Section */}
      <div className="pricing-card-price">
        <span className="price-currency">$</span>
        <span className="price-amount">{pkg.price.amount.toFixed(2)}</span>
        <span className="price-currency-label">USDT</span>
      </div>

      {/* Credits Section */}
      <div className="pricing-card-credits">
        <div className="credit-row">
          <span className="credit-label">Base Credits:</span>
          <span className="credit-value">
            {pkg.credits.amount.toLocaleString()}
          </span>
        </div>

        {pkg.credits.bonusAmount ? (
          <div className="credit-row bonus">
            <span className="credit-label">Bonus Credits:</span>
            <span className="credit-value">
              +{pkg.credits.bonusAmount.toLocaleString()}
            </span>
          </div>
        ) : null}

        <div className="credit-row total">
          <span className="credit-label">Total Credits:</span>
          <span className="credit-value">
            {totalCredits.toLocaleString()}
          </span>
        </div>
      </div>

      {/* Value Proposition */}
      <div className="pricing-card-value">
        <p className="value-text">
          <strong>{creditPerUsd}</strong> credits per USDT
        </p>
      </div>

      {/* CTA Button */}
      <button
        className={`pricing-card-button ${isDisabled ? 'disabled' : ''}`}
        onClick={handlePurchase}
        disabled={isDisabled}
        aria-label={`Purchase ${pkg.name}`}
      >
        {isDisabled ? 'Unavailable' : 'Purchase Now'}
      </button>
    </div>
  )
}

export default PricingCard
