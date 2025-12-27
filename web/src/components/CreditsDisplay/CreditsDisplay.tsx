import { useUserCredits } from '../../hooks/useUserCredits';
import { CreditsIcon } from './CreditsIcon';
import { CreditsValue } from './CreditsValue';
import './credits.module.css';

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
export function CreditsDisplay({ className }: CreditsDisplayProps): React.ReactElement {
  const { credits, loading, error } = useUserCredits();

  // 加载状态：显示骨架屏
  if (loading) {
    return <div className="credits-loading" data-testid="credits-loading" />;
  }

  // 错误状态：显示警告图标和提示
  if (error) {
    return (
      <div
        className="credits-error"
        data-testid="credits-error"
        title="积分加载失败，请刷新页面"
        role="status"
        aria-label="积分加载失败"
      >
        ⚠️
      </div>
    );
  }

  // 无数据：显示占位符（这种情况不应该发生，因为error或loading应该已处理）
  if (!credits) {
    return (
      <div className="credits-error" data-testid="credits-error" title="No credits data">
        -
      </div>
    );
  }

  // 正常状态：显示积分
  return (
    <div
      className={`credits-display ${className || ''}`}
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
