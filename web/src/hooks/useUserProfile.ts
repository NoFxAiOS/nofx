import { useState, useEffect, useCallback } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { useAuth } from '../contexts/AuthContext';

/**
 * 用户资料数据结构
 * 聚合用户基本信息、账户信息和交易员统计
 */
export interface UserProfile {
  id: string;
  email: string;
  created_at: string;
  is_admin: boolean;
  trader_count: number;
  total_equity: number;
  total_pnl: number;
  active_traders: number;
  daily_pnl: number;
  position_count: number;
  last_login_at?: string;
}

/**
 * 用户资料Hook的状态管理
 */
interface UserProfileState {
  userProfile: UserProfile | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * 用户资料数据聚合Hook
 *
 * 架构设计原则：
 * 1. 单一职责 - 只负责用户资料数据的获取和管理
 * 2. 关注点分离 - 数据逻辑与UI逻辑分离
 * 3. 错误边界 - 完善的错误处理和重试机制
 * 4. 性能优化 - 智能缓存和数据聚合
 *
 * 实现特点：
 * - 使用SWR进行数据缓存和自动重验证
 * - 聚合多个API端点的数据
 * - 提供骨架屏加载状态
 * - 支持错误重试和数据刷新
 */
export function useUserProfile(): UserProfileState {
  const { user, token } = useAuth();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [userProfile, setUserProfile] = useState<UserProfile | null>(null);

  /**
   * 数据获取函数
   * 聚合用户基本信息、账户信息和交易员数据
   */
  const fetchUserProfile = useCallback(async () => {
    if (!user || !token) {
      throw new Error('用户未登录');
    }

    try {
      // 并行获取用户账户信息和交易员列表
      const [accountData, tradersData] = await Promise.all([
        api.getAccount(),
        api.getTraders()
      ]);

      // 数据聚合和转换
      const profile: UserProfile = {
        id: user.id || 'unknown',
        email: user.email,
        created_at: new Date().toISOString(), // 这里应该从用户API获取，暂时使用当前时间
        is_admin: false,
        trader_count: tradersData?.length || 0,
        total_equity: accountData?.total_equity || 0,
        total_pnl: accountData?.total_pnl || 0,
        active_traders: tradersData?.filter((t: any) => t.is_running).length || 0,
        daily_pnl: accountData?.daily_pnl || 0,
        position_count: accountData?.position_count || 0,
        last_login_at: localStorage.getItem('last_login_time') || undefined
      };

      return profile;
    } catch (error) {
      console.error('获取用户资料失败:', error);
      throw error;
    }
  }, [user, token]);

  /**
   * 使用SWR进行数据管理
   * 配置智能缓存和重验证策略
   */
  const { data, mutate, error: swrError } = useSWR(
    // 只有当用户登录时才获取数据
    user && token ? 'user-profile' : null,
    fetchUserProfile,
    {
      // 缓存策略：15秒刷新间隔，减少频繁请求
      refreshInterval: 15000,
      // 窗口聚焦时不重验证，避免不必要的请求
      revalidateOnFocus: false,
      // 去重间隔：10秒内相同请求只发送一次
      dedupingInterval: 10000,
      // 错误重试策略
      onError: (error) => {
        console.error('用户资料数据获取失败:', error);
        setError(error.message || '获取用户资料失败');
      },
      onSuccess: (data) => {
        setUserProfile(data);
        setError(null);
      }
    }
  );

  // 同步SWR状态到组件状态
  useEffect(() => {
    if (swrError) {
      setError(swrError.message || '获取用户资料失败');
      setLoading(false);
    } else if (data) {
      setUserProfile(data);
      setLoading(false);
      setError(null);
    } else if (!user || !token) {
      setLoading(false);
      setError(null);
      setUserProfile(null);
    }
  }, [data, swrError, user, token]);

  /**
   * 数据刷新函数
   * 手动触发数据重新获取
   */
  const refetch = useCallback(() => {
    mutate();
  }, [mutate]);

  return {
    userProfile,
    loading: loading && !data && !error,
    error,
    refetch
  };
}

export default useUserProfile;
