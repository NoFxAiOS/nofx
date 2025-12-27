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
      // 记录调试信息
      if (typeof window !== 'undefined') {
        (window as any).__DEBUG_CREDITS_HOOK__ = {
          hasUser: !!user?.id,
          hasToken: !!token,
          userEmail: user?.email,
          timestamp: new Date().toISOString(),
        };
      }
      setCredits(null);
      setError(null);
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      // 记录API请求开始
      if (typeof window !== 'undefined') {
        console.log('[useUserCredits] 发送API请求', {
          url: `${API_BASE}/user/credits`,
          userEmail: user?.email,
          tokenExists: !!token,
        });
      }

      const response = await fetch(`${API_BASE}/user/credits`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        if (response.status === 401) {
          // 认证失败：token无效或已过期
          // 记录错误信息以便调试
          if (typeof window !== 'undefined') {
            console.warn('[useUserCredits] 认证失败 (401)', {
              userEmail: user?.email,
              tokenExists: !!token,
              timestamp: new Date().toISOString(),
            });
          }
          // 设置错误而不是无声清空，这样UI能显示警告
          setError(new Error('认证失败，请重新登录'));
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

      // 后端返回格式: {"code":200,"data":{"available_credits":0,"total_credits":0,"used_credits":0}}
      // 前端期望格式: {"available":0,"total":0,"used":0,"lastUpdated":"..."}
      let creditsData: UserCredits;
      
      if (data.data && typeof data.data === 'object') {
        // 解析嵌套的 data 对象，字段名映射
        const apiData = data.data;
        creditsData = {
          available: typeof apiData.available_credits === 'number' ? apiData.available_credits : 0,
          total: typeof apiData.total_credits === 'number' ? apiData.total_credits : 0,
          used: typeof apiData.used_credits === 'number' ? apiData.used_credits : 0,
          lastUpdated: new Date().toISOString(),
        };
      } else if (typeof data.available === 'number') {
        // 兼容直接返回的格式
        creditsData = data as UserCredits;
      } else {
        throw new Error('API响应数据格式错误: 缺少必要字段');
      }

      // 记录API响应成功
      if (typeof window !== 'undefined') {
        console.log('[useUserCredits] API响应成功', {
          available: creditsData.available,
          total: creditsData.total,
          used: creditsData.used,
        });
        (window as any).__DEBUG_CREDITS_HOOK__ = {
          success: true,
          credits: creditsData,
          timestamp: new Date().toISOString(),
        };
      }

      setCredits(creditsData);
      setLoading(false);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));

      // 记录错误信息用于调试
      if (typeof window !== 'undefined') {
        console.error('[useUserCredits] API请求失败', {
          error: error.message,
          errorType: err instanceof TypeError ? 'TypeError (网络问题)' : 'Other',
          userEmail: user?.email,
          timestamp: new Date().toISOString(),
        });
        (window as any).__DEBUG_CREDITS_HOOK__ = {
          success: false,
          error: error.message,
          errorType: err instanceof TypeError ? 'Network Error' : 'Unknown',
          timestamp: new Date().toISOString(),
        };
      }

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
