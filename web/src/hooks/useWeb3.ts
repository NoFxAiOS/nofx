import { useState, useCallback, useEffect } from 'react';

// ============ 类型定义 ============

interface UseWeb3State {
  address: string | null;
  isConnected: boolean;
  walletType: 'metamask' | 'tp' | null;
  error: string | null;
  isConnecting: boolean;
}

// ConnectOptions - 已移除，未使用

interface AuthResult {
  success: boolean;
  message: string;
  token?: string;
  walletAddr?: string;
  boundWallets?: string[];
}

// ============ 防XSS和CSRF辅助函数 ============

// 清理地址（防止XSS）
const sanitizeAddress = (addr: string): string => {
  // 只保留0x前缀和十六进制字符
  const cleaned = addr.replace(/[^0-9a-fA-Fx]/g, '');
  return cleaned;
};

// 验证地址格式
const isValidAddress = (addr: string): boolean => {
  if (!addr || typeof addr !== 'string') return false;
  if (!addr.startsWith('0x') && !addr.startsWith('0X')) return false;
  if (addr.length !== 42) return false;
  return /^0x[0-9a-fA-F]{40}$/.test(addr);
};

// 清理错误消息（防止信息泄露）
const sanitizeErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    // 只返回用户友好的错误消息
    const msg = error.message;
    if (msg.includes('用户取消')) return '用户取消了操作';
    if (msg.includes('未安装')) return '请安装钱包扩展';
    return '操作失败，请重试';
  }
  return '未知错误';
};

// ============ MetaMask连接 ============

const connectMetaMask = async (): Promise<string> => {
  // 检查MetaMask是否存在
  if (typeof window.ethereum === 'undefined' || !window.ethereum) {
    throw new Error('MetaMask未安装，请访问 https://metamask.io 下载安装');
  }

  // 验证MetaMask版本
  const isMetaMask = window.ethereum.isMetaMask;
  if (!isMetaMask) {
    throw new Error('检测到非MetaMask钱包，请使用MetaMask');
  }

  // 请求连接
  const accounts = await window.ethereum.request({
    method: 'eth_requestAccounts',
  });

  if (!accounts || accounts.length === 0) {
    throw new Error('未获取到钱包地址');
  }

  const address = sanitizeAddress(accounts[0]);
  if (!isValidAddress(address)) {
    throw new Error('获取到的地址格式无效');
  }

  return address;
};

// ============ TP钱包连接 ============

const connectTPWallet = async (): Promise<string> => {
  // 检查TP钱包是否存在
  if (typeof window.ethereum === 'undefined' || !window.ethereum) {
    throw new Error('TP钱包未安装，请从应用商店下载TP钱包');
  }

  // 检查是否为TP钱包
  const isTP = window.ethereum.isTokenPocket || window.ethereum.isTp;
  if (!isTP) {
    throw new Error('检测到非TP钱包，请使用TP钱包');
  }

  // 请求连接
  const accounts = await window.ethereum.request({
    method: 'eth_requestAccounts',
  });

  if (!accounts || accounts.length === 0) {
    throw new Error('未获取到钱包地址');
  }

  const address = sanitizeAddress(accounts[0]);
  if (!isValidAddress(address)) {
    throw new Error('获取到的地址格式无效');
  }

  return address;
};

// ============ 生成签名 ============

const generateSignature = async (
  address: string,
  message: string,
  walletType: 'metamask' | 'tp'
): Promise<string> => {
  // 验证地址和消息
  if (!isValidAddress(address)) {
    throw new Error('地址格式无效');
  }

  if (!message || message.length === 0) {
    throw new Error('签名消息不能为空');
  }

  // 检查钱包是否存在
  if (typeof window.ethereum === 'undefined' || !window.ethereum) {
    throw new Error('未检测到钱包扩展');
  }

  let signature: string;

  try {
    if (walletType === 'metamask') {
      signature = await window.ethereum.request({
        method: 'personal_sign',
        params: [message, address],
      });
    } else if (walletType === 'tp') {
      // TP钱包的签名方法
      signature = await window.ethereum.request({
        method: 'personal_sign',
        params: [address, message],
      });
    } else {
      throw new Error('不支持的钱包类型');
    }
  } catch (error) {
    if (error instanceof Error && error.message.includes('User denied')) {
      throw new Error('用户取消了签名');
    }
    throw new Error('签名失败: ' + sanitizeErrorMessage(error));
  }

  // 验证签名格式
  if (!signature || typeof signature !== 'string') {
    throw new Error('签名格式无效');
  }

  // 清理签名（确保0x前缀）
  if (!signature.startsWith('0x') && !signature.startsWith('0X')) {
    signature = '0x' + signature;
  }

  if (signature.length !== 132) {
    throw new Error('签名长度无效，需要132字符');
  }

  return signature;
};

// ============ Hook实现 ============

export const useWeb3 = () => {
  const [state, setState] = useState<UseWeb3State>({
    address: null,
    isConnected: false,
    walletType: null,
    error: null,
    isConnecting: false,
  });

  // 连接钱包
  const connect = useCallback(async (walletType: 'metamask' | 'tp'): Promise<string> => {
    setState(prev => ({ ...prev, isConnecting: true, error: null }));

    try {
      let address: string;

      if (walletType === 'metamask') {
        address = await connectMetaMask();
      } else if (walletType === 'tp') {
        address = await connectTPWallet();
      } else {
        throw new Error('不支持的钱包类型');
      }

      setState(prev => ({
        ...prev,
        address,
        isConnected: true,
        walletType,
        isConnecting: false,
        error: null,
      }));

      return address;
    } catch (error) {
      const errorMessage = sanitizeErrorMessage(error);
      setState(prev => ({
        ...prev,
        isConnecting: false,
        error: errorMessage,
      }));
      throw new Error(errorMessage);
    }
  }, []);

  // 断开连接
  const disconnect = useCallback(() => {
    setState({
      address: null,
      isConnected: false,
      walletType: null,
      error: null,
      isConnecting: false,
    });
  }, []);

  // 钱包认证
  const authenticate = useCallback(async (): Promise<AuthResult> => {
    if (!state.isConnected || !state.address || !state.walletType) {
      throw new Error('请先连接钱包');
    }

    try {
      // 1. 生成nonce
      const nonceResponse = await fetch('/api/web3/auth/generate-nonce', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          address: state.address,
          wallet_type: state.walletType,
        }),
      });

      if (!nonceResponse.ok) {
        throw new Error('生成nonce失败');
      }

      const { nonce, message } = await nonceResponse.json();

      // 2. 请求签名
      const signature = await generateSignature(
        state.address,
        message,
        state.walletType
      );

      // 3. 发送认证请求
      const authResponse = await fetch('/api/web3/auth/authenticate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          address: state.address,
          signature,
          nonce,
          wallet_type: state.walletType,
        }),
      });

      if (!authResponse.ok) {
        const error = await authResponse.json();
        throw new Error(error.message || '认证失败');
      }

      const result: AuthResult = await authResponse.json();

      return result;
    } catch (error) {
      const errorMessage = sanitizeErrorMessage(error);
      setState(prev => ({ ...prev, error: errorMessage }));
      throw new Error(errorMessage);
    }
  }, [state.isConnected, state.address, state.walletType]);

  // 绑定钱包到用户
  const linkWallet = useCallback(async (isPrimary: boolean = false): Promise<void> => {
    if (!state.isConnected || !state.address || !state.walletType) {
      throw new Error('请先连接钱包');
    }

    try {
      const token = localStorage.getItem('auth_token'); // 从本地存储获取JWT token
      if (!token) {
        throw new Error('请先登录');
      }

      const response = await fetch('/api/web3/wallet/link', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          address: state.address,
          wallet_type: state.walletType,
          is_primary: isPrimary,
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || '绑定钱包失败');
      }
    } catch (error) {
      const errorMessage = sanitizeErrorMessage(error);
      setState(prev => ({ ...prev, error: errorMessage }));
      throw new Error(errorMessage);
    }
  }, [state.isConnected, state.address, state.walletType]);

  // 解绑钱包
  const unlinkWallet = useCallback(async (address: string): Promise<void> => {
    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        throw new Error('请先登录');
      }

      const response = await fetch(`/api/web3/wallet/${address}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || '解绑钱包失败');
      }
    } catch (error) {
      const errorMessage = sanitizeErrorMessage(error);
      setState(prev => ({ ...prev, error: errorMessage }));
      throw new Error(errorMessage);
    }
  }, []);

  // 清除错误
  const clearError = useCallback(() => {
    setState(prev => ({ ...prev, error: null }));
  }, []);

  // 监听钱包地址变化
  useEffect(() => {
    if (typeof window.ethereum === 'undefined' || !window.ethereum) return;

    const handleAccountsChanged = (accounts: string[]) => {
      if (accounts.length === 0) {
        // 用户断开连接
        disconnect();
      } else if (state.address && accounts[0] !== state.address) {
        // 用户切换了账户
        const newAddress = sanitizeAddress(accounts[0]);
        setState(prev => ({
          ...prev,
          address: newAddress,
          isConnected: true,
        }));
      }
    };

    const handleChainChanged = () => {
      // 网络变化时可能需要重新认证
      setState(prev => ({
        ...prev,
        error: '网络已切换，请重新连接',
      }));
    };

    window.ethereum?.on?.('accountsChanged', handleAccountsChanged);
    window.ethereum?.on?.('chainChanged', handleChainChanged);

    return () => {
      window.ethereum?.removeListener?.('accountsChanged', handleAccountsChanged);
      window.ethereum?.removeListener?.('chainChanged', handleChainChanged);
    };
  }, [state.address, disconnect]);

  return {
    // 状态
    address: state.address,
    isConnected: state.isConnected,
    walletType: state.walletType,
    error: state.error,
    isConnecting: state.isConnecting,

    // 方法
    connect,
    disconnect,
    authenticate,
    linkWallet,
    unlinkWallet,
    clearError,
  };
};

// ============ 全局类型声明 ============

declare global {
  interface Window {
    ethereum?: {
      isMetaMask?: boolean;
      isTokenPocket?: boolean;
      isTp?: boolean;
      request: (args: { method: string; params?: any[] }) => Promise<any>;
      on?: (event: string, callback: (...args: any[]) => void) => void;
      removeListener?: (event: string, callback: (...args: any[]) => void) => void;
    };
  }
}
