import { useState } from 'react';
import { useLanguage } from '../contexts/LanguageContext';
import { useAuth } from '../contexts/AuthContext';
import { t } from '../i18n/translations';
import { CreditsDisplay } from './CreditsDisplay';
import { PaymentModal } from '../features/payment/components/PaymentModal';

interface HeaderProps {
  simple?: boolean; // For login/register pages
}

export function Header({ simple = false }: HeaderProps) {
  const { language, setLanguage } = useLanguage();
  const { user } = useAuth();
  const [isPaymentModalOpen, setIsPaymentModalOpen] = useState(false);

  return (
    <header className="glass sticky top-0 z-50 backdrop-blur-xl">
      <div className="max-w-[1920px] mx-auto px-6 py-4">
        <div className="flex items-center justify-between">
          {/* Left - Logo and Title */}
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <img src="/icons/Monnaire_Logo.svg" alt="Monnaire Logo" className="w-8 h-8" />
            </div>
            <div>
              <h1 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                {t('appTitle', language)}
              </h1>
              {!simple && (
                <p className="text-xs mono" style={{ color: '#848E9C' }}>
                  {t('subtitle', language)}
                </p>
              )}
            </div>
          </div>

          {/* Right - User Info, Credits Display and Language Toggle */}
          <div className="flex items-center gap-4">
            {/* User Name - 仅在非简化模式下显示 */}
            {!simple && user?.email && (
              <div className="text-sm" style={{ color: '#848E9C' }}>
                {user.email}
              </div>
            )}

            {/* Credits Display - 仅在非简化模式下显示 */}
            {!simple && <CreditsDisplay />}

            {/* Credits Packages Button - 新增 */}
            <button
              onClick={() => setIsPaymentModalOpen(true)}
              className="px-4 py-2 rounded text-sm font-semibold transition-all"
              style={{
                background: '#007bff',
                color: 'white',
                border: 'none',
                cursor: 'pointer',
                borderRadius: '4px'
              }}
              onMouseEnter={(e) => e.currentTarget.style.background = '#0056b3'}
              onMouseLeave={(e) => e.currentTarget.style.background = '#007bff'}
              aria-label={language === 'zh' ? '打开用户积分套餐购买面板' : 'Open credit packages'}
              title={language === 'zh' ? '点击购买更多积分' : 'Click to purchase credits'}
            >
              {language === 'zh' ? '积分套餐' : 'Packages'}
            </button>

            {/* Language Toggle (always show) */}
            <div className="flex gap-1 rounded p-1" style={{ background: '#1E2329' }}>
              <button
                onClick={() => setLanguage('zh')}
                className="px-3 py-1.5 rounded text-xs font-semibold transition-all"
                style={language === 'zh'
                  ? { background: '#F0B90B', color: '#000' }
                  : { background: 'transparent', color: '#848E9C' }
                }
                aria-label="Switch to Chinese"
                aria-current={language === 'zh' ? 'true' : 'false'}
                title={language === 'zh' ? 'Chinese (Current)' : 'Switch to Chinese'}
              >
                中文
              </button>
              <button
                onClick={() => setLanguage('en')}
                className="px-3 py-1.5 rounded text-xs font-semibold transition-all"
                style={language === 'en'
                  ? { background: '#F0B90B', color: '#000' }
                  : { background: 'transparent', color: '#848E9C' }
                }
                aria-label="Switch to English"
                aria-current={language === 'en' ? 'true' : 'false'}
                title={language === 'en' ? 'English (Current)' : 'Switch to English'}
              >
                EN
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Payment Modal */}
      <PaymentModal
        isOpen={isPaymentModalOpen}
        onClose={() => setIsPaymentModalOpen(false)}
      />
    </header>
  );
}
