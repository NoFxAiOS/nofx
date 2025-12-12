import React, { createContext, useContext, useState, useEffect } from 'react';
import { getApiBaseUrl } from '../lib/apiConfig';

// 统一的 API 基础 URL
const API_BASE = getApiBaseUrl();

interface User {
  id: string;
  email: string;
  invite_code?: string;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<{ success: boolean; message?: string; userID?: string; requiresOTP?: boolean }>;
  register: (email: string, password: string, betaCode?: string, inviteCode?: string) => Promise<{ success: boolean; message?: string; userID?: string; otpSecret?: string; qrCodeURL?: string }>;
  verifyOTP: (userID: string, otpCode: string) => Promise<{ success: boolean; message?: string }>;
  completeRegistration: (userID: string, otpCode: string) => Promise<{ success: boolean; message?: string }>;
  requestPasswordReset: (email: string) => Promise<{ success: boolean; message?: string }>;
  resetPassword: (token: string, password: string, otpCode: string) => Promise<{ success: boolean; message?: string }>;
  logout: () => void;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const logout = () => {
    setUser(null);
    setToken(null);
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
  };

  const fetchCurrentUser = async (currentToken: string) => {
    try {
      const response = await fetch(`${API_BASE}/user/me`, {
        headers: {
          'Authorization': `Bearer ${currentToken}`,
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        const data = await response.json();
        // Update user state and local storage with latest data
        const userInfo: User = { 
          id: data.id, 
          email: data.email, 
          invite_code: data.invite_code 
        };
        setUser(userInfo);
        localStorage.setItem('auth_user', JSON.stringify(userInfo));
      } else if (response.status === 401) {
        // Token expired or invalid
        logout();
      }
    } catch (error) {
      console.error('Failed to refresh user profile:', error);
    }
  };

  useEffect(() => {
    // 检查本地存储中是否有有效的认证信息
    const savedToken = localStorage.getItem('auth_token');
    const savedUser = localStorage.getItem('auth_user');

    if (savedToken && savedUser) {
      try {
        // 验证JWT token的有效性
        if (isValidToken(savedToken)) {
          setToken(savedToken);
          setUser(JSON.parse(savedUser));
          // Refresh user profile from backend to get latest fields (e.g. invite_code)
          fetchCurrentUser(savedToken);
        } else {
          console.warn('Stored token is invalid or expired');
          // 清除无效数据
          logout();
        }
      } catch (error) {
        console.error('Failed to parse saved user data:', error);
        // 清除无效数据
        logout();
      }
    }
    setIsLoading(false);
  }, []);

  // 辅助函数：验证JWT token格式和过期时间
  const isValidToken = (token: string): boolean => {
    try {
      // JWT格式: header.payload.signature
      const parts = token.split('.');
      if (parts.length !== 3) {
        return false;
      }

      // 解析payload (base64解码)
      const payload = JSON.parse(atob(parts[1]));

      // 检查过期时间
      if (payload.exp && Date.now() >= payload.exp * 1000) {
        console.log('Token expired');
        return false;
      }

      return true;
    } catch (error) {
      console.error('Token validation error:', error);
      return false;
    }
  };

  const login = async (email: string, password: string) => {
    try {
      const response = await fetch(`${API_BASE}/login`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      const data = await response.json();

      if (response.ok) {
        // 登录成功，保存token和用户信息
        const userInfo: User = { 
          id: data.user_id, 
          email: data.email,
          invite_code: data.invite_code 
        };
        setToken(data.token);
        setUser(userInfo);
        localStorage.setItem('auth_token', data.token);
        localStorage.setItem('auth_user', JSON.stringify(userInfo));

        return {
          success: true,
          message: data.message,
        };
      } else {
        return { success: false, message: data.error };
      }
    } catch (error) {
      return { success: false, message: '登录失败，请重试' };
    }
  };

  const register = async (email: string, password: string, betaCode?: string, inviteCode?: string) => {
    try {
      const requestBody: { email: string; password: string; beta_code?: string; invite_code?: string } = { email, password };
      if (betaCode) {
        requestBody.beta_code = betaCode;
      }
      if (inviteCode) {
        requestBody.invite_code = inviteCode;
      }

      const response = await fetch(`${API_BASE}/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });

      const data = await response.json();

      // 检查业务逻辑是否成功（而不是HTTP状态）
      if (data.success) {
        // 注册成功，自动登录
        const userInfo: User = { 
          id: data.user.id, 
          email: data.user.email,
          invite_code: data.user.invite_code
        };
        setToken(data.token);
        setUser(userInfo);
        localStorage.setItem('auth_token', data.token);
        localStorage.setItem('auth_user', JSON.stringify(userInfo));

        return {
          success: true,
          message: data.message,
        };
      } else {
        // 注册失败，返回详细错误信息
        return {
          success: false,
          message: data.error,
          details: data.details,
        };
      }
    } catch (error) {
      return {
        success: false,
        message: '注册失败',
        details: '网络错误，请检查网络连接后重试',
      };
    }
  };

  const verifyOTP = async (userID: string, otpCode: string) => {
    try {
      const response = await fetch(`${API_BASE}/verify-otp`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      });

      const data = await response.json();

      if (response.ok) {
        // 登录成功，保存token和用户信息
        const userInfo = { id: data.user_id, email: data.email };
        setToken(data.token);
        setUser(userInfo);
        localStorage.setItem('auth_token', data.token);
        localStorage.setItem('auth_user', JSON.stringify(userInfo));
        
        // 跳转到首页
        window.history.pushState({}, '', '/');
        window.dispatchEvent(new PopStateEvent('popstate'));
        
        return { success: true, message: data.message };
      } else {
        return { success: false, message: data.error };
      }
    } catch (error) {
      return { success: false, message: 'OTP验证失败，请重试' };
    }
  };

  const completeRegistration = async (userID: string, otpCode: string) => {
    try {

      const response = await fetch(`${API_BASE}/complete-registration`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      });

      const data = await response.json();

      if (response.ok) {
        // 注册完成，自动登录
        const userInfo = { id: data.user_id, email: data.email };
        setToken(data.token);
        setUser(userInfo);
        localStorage.setItem('auth_token', data.token);
        localStorage.setItem('auth_user', JSON.stringify(userInfo));

        // 跳转到首页
        window.history.pushState({}, '', '/');
        window.dispatchEvent(new PopStateEvent('popstate'));

        return { success: true, message: data.message };
      } else {
        return { success: false, message: data.error };
      }
    } catch (error) {
      return { success: false, message: '注册完成失败，请重试' };
    }
  };

  const requestPasswordReset = async (email: string) => {
    try {

      const response = await fetch(`${API_BASE}/request-password-reset`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email }),
      });

      const data = await response.json();

      if (response.ok) {
        return { success: true, message: data.message };
      } else {
        return { success: false, message: data.error };
      }
    } catch (error) {
      return { success: false, message: '请求失败，请重试' };
    }
  };

  const resetPassword = async (token: string, password: string, otpCode: string) => {
    try {

      const response = await fetch(`${API_BASE}/reset-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ token, password, otp_code: otpCode }),
      });

      const data = await response.json();

      if (response.ok) {
        return { success: true, message: data.message };
      } else {
        return { success: false, message: data.error };
      }
    } catch (error) {
      return { success: false, message: '密码重置失败，请重试' };
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        register,
        verifyOTP,
        completeRegistration,
        requestPasswordReset,
        resetPassword,
        logout,
        isLoading,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}