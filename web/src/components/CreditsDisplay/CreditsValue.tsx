import { useLanguage } from '../../contexts/LanguageContext';
import styles from './credits.module.css';

/**
 * CreditsValue 组件属性
 */
export interface CreditsValueProps {
  value: number;
  format?: 'number' | 'short';
  onOpen?: () => void;
  disabled?: boolean;
  loading?: boolean;
}

/**
 * 格式化大数字为简化形式
 * 例如：1000 -> 1k, 1500000 -> 1.5M
 */
function formatShortNumber(num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toString();
}

/**
 * CreditsValue - 积分数值组件
 * 点击打开支付modal，允许用户购买积分
 */
export function CreditsValue({
  value,
  format = 'number',
  onOpen,
  disabled = false,
  loading = false,
}: CreditsValueProps): React.ReactElement {
  const { language } = useLanguage();
  const displayValue = format === 'short' ? formatShortNumber(value) : value;
  const creditsLabel = language === 'zh' ? '用户积分' : 'Credits';

  const handleClick = () => {
    if (!disabled && !loading) {
      window.location.href = 'https://www.agentrade.xyz/profile';
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.key === 'Enter' || e.key === ' ') && !disabled && !loading) {
      e.preventDefault();
      handleClick();
    }
  };

  return (
    <span
      className={styles.creditsValue}
      data-testid="credits-value"
      data-value={value}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      style={{
        cursor: disabled || loading ? 'not-allowed' : 'pointer',
        opacity: disabled ? 0.6 : 1,
      }}
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-label={`${displayValue} ${creditsLabel}. ${!disabled ? 'Click to purchase more credits' : 'Credits display'}`}
      aria-disabled={disabled || loading}
      aria-busy={loading}
    >
      {loading && <span className={styles.spinner}>⟳ </span>}
      {displayValue}({creditsLabel})
    </span>
  );
}