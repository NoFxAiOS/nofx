import { useUserCredits } from '../../hooks/useUserCredits';
import { useAuth } from '../../contexts/AuthContext';
import { CreditsIcon } from './CreditsIcon';
import { CreditsValue } from './CreditsValue';
import styles from './credits.module.css';

/**
 * CreditsDisplay 组件属性
 */
export interface CreditsDisplayProps {
  className?: string;
}

/**
 * CreditsDisplay - 用户积分显示组件
 *
 * 位置：Header右侧，语言切换按钮左边
 * 职责：
 * - 显示用户剩余积分
 * - 处理加载、错误和正常状态
 * - 集成useUserCredits hook管理数据
 *
 * @param {CreditsDisplayProps} props - 组件属性
 * @returns {React.ReactElement} 积分显示组件
 *
 * @example
 * <CreditsDisplay />
 * // 或带自定义className
 * <CreditsDisplay className="custom-class" />
 */
export function CreditsDisplay({ className }: CreditsDisplayProps): React.ReactElement | null {
  const { user, token, isLoading: authLoading } = useAuth();
  const { credits, loading, error } = useUserCredits();

  // Development-only logging for debugging
  if (process.env.NODE_ENV === 'development') {
    console.debug('[CreditsDisplay] State:', {
      hasAuth: !!(user?.id && token),
      authLoading,
      creditsAvailable: credits?.available,
      loading,
      hasError: !!error
    });
  }

  // 如果没有用户ID或token，不显示（未登录状态）
  if (!user?.id || !token) {
    return null;
  }

  // 认证加载中或积分加载中：显示骨架屏
  if (authLoading || loading) {
    return <div className={styles.creditsLoading} data-testid="credits-loading" />;
  }

  // 错误状态：显示警告图标和提示
  if (error) {
    if (process.env.NODE_ENV === 'development') {
      console.debug('[CreditsDisplay] Error loading credits:', error.message);
    }
    return (
      <div
        className={styles.creditsError}
        data-testid="credits-error"
        title="积分加载失败，请刷新页面"
        role="status"
        aria-label="积分加载失败"
      >
        ⚠️
      </div>
    );
  }

  // 无数据：显示0积分
  if (!credits) {
    return (
      <div
        className={`${styles.creditsDisplay} ${className || ''}`}
        data-testid="credits-display"
        role="status"
        aria-live="polite"
        aria-label="Available credits: 0"
      >
        <CreditsIcon />
        <CreditsValue value={0} />
      </div>
    );
  }

  // 正常状态：显示积分
  return (
    <div
      className={`${styles.creditsDisplay} ${className || ''}`}
      data-testid="credits-display"
      role="status"
      aria-live="polite"
      aria-label={`Available credits: ${credits.available}`}
    >
      <CreditsIcon />
      <CreditsValue value={credits.available} />
    </div>
  );
}
