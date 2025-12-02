/**
 * WalletSelector组件
 * 钱包选择弹窗，支持MetaMask和TP钱包
 * 遵循UI设计规范: openspec/proposals/connect-web3-wallet-button
 */

import { useState } from 'react';
import { motion } from 'framer-motion';
import { X, ExternalLink, AlertCircle, CheckCircle } from 'lucide-react';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { detectMetaMask, detectTPWallet } from '../utils/walletDetector';

interface WalletOption {
  id: 'metamask' | 'tp';
  name: string;
  icon: string;
  description: string;
  isInstalled: boolean;
  confidence: number;
  installUrl?: string;
}

interface WalletSelectorProps {
  onSelect: (walletType: 'metamask' | 'tp') => Promise<void>;
  onClose: () => void;
}

export function WalletSelector({ onSelect, onClose }: WalletSelectorProps) {
  const { language } = useLanguage();
  const [selectedWallet, setSelectedWallet] = useState<'metamask' | 'tp' | null>(null);
  const [isConnecting, setIsConnecting] = useState(false);

  // 检测已安装的钱包
  const [walletStatus] = useState<WalletOption[]>(() => {
    const metaMask = detectMetaMask();
    const tpWallet = detectTPWallet();

    return [
      {
        id: 'metamask',
        name: 'MetaMask',
        icon: '🦊',
        description: t('web3.metaMaskDesc', language) || '最流行的以太坊浏览器钱包',
        isInstalled: metaMask.isDetected,
        confidence: metaMask.confidence,
        installUrl: 'https://metamask.io/download',
      },
      {
        id: 'tp',
        name: 'TP钱包',
        icon: '🔵',
        description: t('web3.tpWalletDesc', language) || '安全可靠的数字钱包',
        isInstalled: tpWallet.isDetected,
        confidence: tpWallet.confidence,
        installUrl: 'https://www.tokenpocket.pro/',
      },
    ];
  });

  // 处理钱包选择
  const handleWalletSelect = async (walletType: 'metamask' | 'tp') => {
    const wallet = walletStatus.find(w => w.id === walletType);
    if (!wallet) return;

    // 如果钱包未安装，显示安装提示
    if (!wallet.isInstalled) {
      return;
    }

    setSelectedWallet(walletType);
    setIsConnecting(true);

    try {
      await onSelect(walletType);
      // 连接成功后弹窗会自动关闭
    } catch (error) {
      console.error('钱包连接失败:', error);
      setSelectedWallet(null);
    } finally {
      setIsConnecting(false);
    }
  };

  // 处理安装链接
  const handleInstallClick = (wallet: WalletOption) => {
    if (wallet.installUrl) {
      window.open(wallet.installUrl, '_blank', 'noopener,noreferrer');
    }
  };

  // 获取安装状态颜色
  const getInstallStatusColor = (isInstalled: boolean) => {
    return isInstalled ? 'text-green-500' : 'text-orange-500';
  };

  // 获取安装状态文字
  const getInstallStatusText = (isInstalled: boolean) => {
    return isInstalled
      ? (t('web3.installed', language) || '已安装')
      : (t('web3.notInstalled', language) || '未安装');
  };

  return (
    <div
      className="rounded-xl shadow-2xl overflow-hidden"
      style={{
        background: 'var(--brand-dark-gray)',
        border: '1px solid var(--panel-border)',
        minWidth: '360px',
        maxWidth: '90vw',
      }}
      role="dialog"
      aria-modal="true"
      aria-labelledby="wallet-selector-title"
    >
      {/* 标题栏 */}
      <div
        className="flex items-center justify-between px-6 py-4"
        style={{ borderBottom: '1px solid var(--panel-border)' }}
      >
        <h2 id="wallet-selector-title" className="text-lg font-semibold" style={{ color: 'var(--brand-light-gray)' }}>
          {t('web3.selectWallet', language) || '选择您的钱包类型'}
        </h2>
        <button
          onClick={onClose}
          className="p-1 rounded hover:opacity-80 transition-opacity"
          style={{ color: 'var(--brand-light-gray)' }}
          aria-label={t('common.close', language) || '关闭'}
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* 钱包选项列表 */}
      <div className="p-4 space-y-3">
        {walletStatus.map((wallet) => (
          <motion.button
            key={wallet.id}
            onClick={() => handleWalletSelect(wallet.id)}
            disabled={!wallet.isInstalled || isConnecting}
            whileHover={{ scale: wallet.isInstalled ? 1.02 : 1 }}
            whileTap={{ scale: wallet.isInstalled ? 0.98 : 1 }}
            className={`
              w-full text-left p-4 rounded-lg border transition-all duration-200
              ${selectedWallet === wallet.id ? 'border-yellow-500 bg-yellow-500/5' : 'border-transparent hover:border-panel-border'}
              ${!wallet.isInstalled ? 'opacity-75' : ''}
              ${isConnecting ? 'cursor-not-allowed' : 'cursor-pointer'}
            `}
            style={{
              background: 'var(--panel-bg)',
              borderColor: selectedWallet === wallet.id ? 'var(--brand-yellow)' : 'var(--panel-border)',
            }}
          >
            <div className="flex items-start gap-4">
              {/* 钱包图标 */}
              <div className="flex-shrink-0">
                <div
                  className="w-12 h-12 rounded-full flex items-center justify-center text-2xl"
                  style={{ background: 'rgba(255, 255, 255, 0.05)' }}
                >
                  {wallet.icon}
                </div>
              </div>

              {/* 钱包信息 */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="font-semibold text-base" style={{ color: 'var(--brand-light-gray)' }}>
                    {wallet.name}
                  </h3>
                  {isConnecting && selectedWallet === wallet.id && (
                    <div className="animate-pulse">
                      <CheckCircle className="w-5 h-5 text-yellow-500" />
                    </div>
                  )}
                </div>

                <p className="text-sm mb-2" style={{ color: 'var(--text-secondary)' }}>
                  {wallet.description}
                </p>

                <div className="flex items-center justify-between">
                  {/* 安装状态 */}
                  <div className="flex items-center gap-2">
                    <span className={`text-xs font-medium ${getInstallStatusColor(wallet.isInstalled)}`}>
                      {getInstallStatusText(wallet.isInstalled)}
                    </span>
                    {!wallet.isInstalled && wallet.installUrl && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleInstallClick(wallet);
                        }}
                        className="flex items-center gap-1 text-xs text-yellow-500 hover:text-yellow-400 transition-colors"
                      >
                        <ExternalLink className="w-3 h-3" />
                        {t('web3.install', language) || '安装'}
                      </button>
                    )}
                  </div>

                  {/* 置信度指示器 */}
                  {wallet.isInstalled && (
                    <div className="flex items-center gap-1">
                      <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        {t('web3.confidence', language) || '置信度'}:
                      </span>
                      <span
                        className="text-xs font-medium"
                        style={{
                          color:
                            wallet.confidence >= 90
                              ? 'var(--brand-green)'
                              : wallet.confidence >= 70
                              ? 'var(--brand-yellow)'
                              : 'var(--binance-orange)',
                        }}
                      >
                        {wallet.confidence}%
                      </span>
                    </div>
                  )}
                </div>

                {/* 未安装提示 */}
                {!wallet.isInstalled && (
                  <div className="mt-3 p-2 rounded" style={{ background: 'rgba(255, 165, 0, 0.1)' }}>
                    <div className="flex items-start gap-2">
                      <AlertCircle className="w-4 h-4 text-orange-500 flex-shrink-0 mt-0.5" />
                      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        {t('web3.installPrompt', language) ||
                          `请先安装 ${wallet.name} 钱包插件，然后刷新页面重试`}
                      </p>
                    </div>
                  </div>
                )}

                {/* 连接中提示 */}
                {isConnecting && selectedWallet === wallet.id && (
                  <div className="mt-3 p-2 rounded bg-blue-500/10">
                    <div className="flex items-center gap-2">
                      <div className="animate-spin rounded-full h-4 w-4 border-2 border-blue-500 border-t-transparent" />
                      <p className="text-xs" style={{ color: 'var(--brand-blue)' }}>
                        {t('web3.connectingWallet', language) || '正在连接钱包...'}
                      </p>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </motion.button>
        ))}

        {/* 更多信息链接 */}
        <div className="mt-4 pt-4 border-t" style={{ borderColor: 'var(--panel-border)' }}>
          <p className="text-xs text-center" style={{ color: 'var(--text-secondary)' }}>
            {t('web3.secureConnection', language) || '所有连接都是安全加密的，我们不会存储您的私钥'}
          </p>
          <div className="flex justify-center gap-4 mt-2">
            <a
              href="https://metamask.io/security"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-yellow-500 hover:text-yellow-400 transition-colors"
            >
              {t('web3.securityInfo', language) || '安全信息'}
            </a>
            <a
              href="https://docs.metamask.io/"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-yellow-500 hover:text-yellow-400 transition-colors"
            >
              {t('web3.help', language) || '帮助文档'}
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}

export default WalletSelector;
