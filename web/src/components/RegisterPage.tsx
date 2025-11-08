import React, { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { Smartphone, Lock, Eye, EyeOff } from 'lucide-react';
import { Input } from './ui/input';
import PasswordChecklist from 'react-password-checklist';

export function RegisterPage() {
  const { language } = useLanguage();
  const { register, completeRegistration } = useAuth();
  const [step, setStep] = useState<'register' | 'setup-otp' | 'verify-otp'>('register');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [passwordValid, setPasswordValid] = useState(false);
  const [otpCode, setOtpCode] = useState('');
  const [userID, setUserID] = useState('');
  const [otpSecret, setOtpSecret] = useState('');
  const [qrCodeURL, setQrCodeURL] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (password !== confirmPassword) {
      setError(t('passwordMismatch', language));
      return;
    }

    if (!passwordValid) {
      setError(t('passwordNotMeetRequirements', language));
      return;
    }

    setLoading(true);

    const result = await register(email, password);

    if (result.success && result.userID) {
      setUserID(result.userID);
      setOtpSecret(result.otpSecret || '');
      setQrCodeURL(result.qrCodeURL || '');
      setStep('setup-otp');
    } else {
      setError(result.message || t('registrationFailed', language));
    }

    setLoading(false);
  };

  const handleSetupComplete = () => {
    setStep('verify-otp');
  };

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const result = await completeRegistration(userID, otpCode);

    if (!result.success) {
      setError(result.message || t('registrationFailed', language));
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ background: '#0B0E11' }}>
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 flex items-center justify-center">
            <img src="/images/logo.png" alt="NoFx Logo" className="w-full h-full object-contain" />
          </div>
          <h1 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
            {t('appTitle', language)}
          </h1>
          <p className="text-sm mt-2" style={{ color: '#848E9C' }}>
            {step === 'register' && t('registerTitle', language)}
            {step === 'setup-otp' && t('setupTwoFactor', language)}
            {step === 'verify-otp' && t('verifyOTP', language)}
          </p>
        </div>

        {/* Registration Form */}
        <div className="rounded-lg p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
          {step === 'register' && (
            <form onSubmit={handleRegister} className="space-y-4">
              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  {t('email', language)}
                </label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t('emailPlaceholder', language)}
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  {t('password', language)}
                </label>
                <div className="relative">
                  <Input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="pr-10"
                    placeholder={t('passwordPlaceholder', language)}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2"
                    style={{ color: '#848E9C' }}
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  {t('confirmPassword', language)}
                </label>
                <div className="relative">
                  <Input
                    type={showConfirmPassword ? 'text' : 'password'}
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className="pr-10"
                    placeholder={t('confirmPasswordPlaceholder', language)}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2"
                    style={{ color: '#848E9C' }}
                  >
                    {showConfirmPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
              </div>

              {password && (
                <PasswordChecklist
                  rules={['minLength', 'lowercase', 'uppercase', 'number', 'specialChar', 'match']}
                  minLength={8}
                  value={password}
                  valueAgain={confirmPassword}
                  onChange={(isValid) => setPasswordValid(isValid)}
                  messages={{
                    minLength: t('passwordMinLength', language),
                    lowercase: t('passwordLowercase', language),
                    uppercase: t('passwordUppercase', language),
                    number: t('passwordNumber', language),
                    specialChar: t('passwordSpecialChar', language),
                    match: t('passwordMatch', language),
                  }}
                  style={{ fontSize: '12px', color: '#848E9C' }}
                  iconSize={12}
                />
              )}

              {error && (
                <div className="text-sm px-3 py-2 rounded" style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}>
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading || !passwordValid}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                {loading ? t('loading', language) : t('registerButton', language)}
              </button>
            </form>
          )}

          {step === 'setup-otp' && (
            <div className="space-y-4">
              <div className="text-center">
                <div className="mb-2 flex justify-center">
                  <Smartphone className="w-10 h-10" style={{ color: '#F0B90B' }} />
                </div>
                <h3 className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  {t('setupTwoFactor', language)}
                </h3>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  {t('setupTwoFactorDesc', language)}
                </p>
              </div>

              <div className="space-y-3">
                <div className="p-3 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                  <p className="text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                    {t('step1Title', language)}
                  </p>
                  <p className="text-xs" style={{ color: '#848E9C' }}>
                    {t('step1Desc', language)}
                  </p>
                </div>

                <div className="p-3 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                  <p className="text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                    {t('step2Title', language)}
                  </p>
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {t('step2Desc', language)}
                  </p>

                  {qrCodeURL && (
                    <div className="mt-2">
                      <p className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('qrCodeHint', language)}</p>
                      <div className="bg-white p-2 rounded text-center">
                        <img src={`https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=${encodeURIComponent(qrCodeURL)}`}
                             alt="QR Code" className="mx-auto" />
                      </div>
                    </div>
                  )}

                  <div className="mt-2">
                    <p className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('otpSecret', language)}</p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 px-2 py-1 text-xs rounded font-mono"
                            style={{ background: '#2B3139', color: '#EAECEF' }}>
                        {otpSecret}
                      </code>
                      <button
                        onClick={() => copyToClipboard(otpSecret)}
                        className="px-2 py-1 text-xs rounded"
                        style={{ background: '#F0B90B', color: '#000' }}
                      >
                        {t('copy', language)}
                      </button>
                    </div>
                  </div>
                </div>

                <div className="p-3 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                  <p className="text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                    {t('step3Title', language)}
                  </p>
                  <p className="text-xs" style={{ color: '#848E9C' }}>
                    {t('step3Desc', language)}
                  </p>
                </div>
              </div>

              <button
                onClick={handleSetupComplete}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                {t('setupCompleteContinue', language)}
              </button>
            </div>
          )}

          {step === 'verify-otp' && (
            <form onSubmit={handleOTPVerify} className="space-y-4">
              <div className="text-center mb-4">
                <div className="mb-2 flex justify-center">
                  <Lock className="w-10 h-10" style={{ color: '#F0B90B' }} />
                </div>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  {t('enterOTPCode', language)}<br />
                  {t('completeRegistrationSubtitle', language)}
                </p>
              </div>

              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  {t('otpCode', language)}
                </label>
                <input
                  type="text"
                  value={otpCode}
                  onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  placeholder={t('otpPlaceholder', language)}
                  maxLength={6}
                  required
                />
              </div>

              {error && (
                <div className="text-sm px-3 py-2 rounded" style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}>
                  {error}
                </div>
              )}

              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => setStep('setup-otp')}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold"
                  style={{ background: '#2B3139', color: '#848E9C' }}
                >
                  {t('back', language)}
                </button>
                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                  style={{ background: '#F0B90B', color: '#000' }}
                >
                  {loading ? t('loading', language) : t('completeRegistration', language)}
                </button>
              </div>
            </form>
          )}
        </div>

        {/* Login Link */}
        {step === 'register' && (
          <div className="text-center mt-6">
            <p className="text-sm" style={{ color: '#848E9C' }}>
              已有账户？{' '}
              <button
                onClick={() => {
                  window.history.pushState({}, '', '/login');
                  window.dispatchEvent(new PopStateEvent('popstate'));
                }}
                className="font-semibold hover:underline"
                style={{ color: '#F0B90B' }}
              >
                立即登录
              </button>
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
