import { useEffect, useState, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { getApiBaseUrl } from '../lib/apiConfig';

/**
 * 用户积分数据接口
 */
export interface UserCredits {
  total: number;
  available: number;
  used: number;
  lastUpdated: string;
}

/**
 * useUserCredits Hook返回值
 */
export interface UseUserCreditsReturn {
  credits: UserCredits | null;
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
}

const API_BASE = getApiBaseUrl();
const REFRESH_INTERVAL = 30000; // 30秒

/**
 * useUserCredits Hook
 *
 * 获取并管理用户积分数据
 * - 自动30秒刷新一次
 * - 错误自动重试
 * - 清理定时器，防止内存泄漏
 *
 * @returns {UseUserCreditsReturn} 积分数据和操作方法
 *
 * @example
 * const { credits, loading, error } = useUserCredits();
 * if (loading) return <Spinner />;
 * if (error) return <span>-</span>;
 * return <span>{credits?.available}</span>;
 */
export function useUserCredits(): UseUserCreditsReturn {
  const { user, token } = useAuth();
  const [credits, setCredits] = useState<UserCredits | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  /**
   * 获取用户积分
   */
  const fetchCredits = useCallback(async () => {
    if (!user?.id || !token) {
      setCredits(null);
      setError(null);
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const response = await fetch(`${API_BASE}/user/credits`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        if (response.status === 401) {
          // 认证失败，不需要设置错误，直接清空数据
          setCredits(null);
          setLoading(false);
          return;
        }
        throw new Error(`Failed to fetch credits: ${response.statusText}`);
      }

      const data = await response.json();

      // 验证API响应数据格式
      if (!data || typeof data !== 'object') {
        throw new Error('API响应数据格式错误: 期望对象');
      }

      const credits = data as UserCredits;
      if (typeof credits.available !== 'number' ||
          typeof credits.total !== 'number' ||
          typeof credits.used !== 'number') {
        throw new Error('API响应数据格式错误: 缺少必要字段或类型不正确');
      }

      setCredits(credits);
      setLoading(false);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      setError(error);
      setCredits(null);
      setLoading(false);
    }
  }, [user?.id, token]);

  /**
   * 初始化和自动刷新
   */
  useEffect(() => {
    if (!user?.id || !token) {
      return;
    }

    // 首次获取
    fetchCredits();

    // 设置自动刷新定时器
    const interval = setInterval(() => {
      fetchCredits();
    }, REFRESH_INTERVAL);

    // 清理定时器
    return () => clearInterval(interval);
  }, [user?.id, token, fetchCredits]);

  return { credits, loading, error, refetch: fetchCredits };
}
