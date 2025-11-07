import { useState, useEffect } from 'react';
import type { AIModel, Exchange, CreateTraderRequest } from '../types';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';

// WebSocket 流數限制常量
const MAX_SYMBOLS = 250; // Binance WebSocket 限制：1024 流 / 4 時間週期 = 250 幣種
const WARNING_THRESHOLD = 200; // 接近上限時顯示警告

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_');
  return parts.length > 1 ? parts[parts.length - 1] : fullName;
}

interface TraderConfigData {
  trader_id?: string;
  trader_name: string;
  ai_model: string;
  exchange_id: string;
  btc_eth_leverage: number;
  altcoin_leverage: number;
  trading_symbols: string;
  custom_prompt: string;
  override_base_prompt: boolean;
  system_prompt_template: string;
  is_cross_margin: boolean;
  use_coin_pool: boolean;
  use_oi_top: boolean;
  initial_balance: number;
  scan_interval_minutes: number;
}

interface TraderConfigModalProps {
  isOpen: boolean;
  onClose: () => void;
  traderData?: TraderConfigData | null;
  isEditMode?: boolean;
  availableModels?: AIModel[];
  availableExchanges?: Exchange[];
  onSave?: (data: CreateTraderRequest) => Promise<void>;
}

export function TraderConfigModal({ 
  isOpen, 
  onClose, 
  traderData, 
  isEditMode = false,
  availableModels = [],
  availableExchanges = [],
  onSave
}: TraderConfigModalProps) {
  const { language } = useLanguage();
  const [formData, setFormData] = useState<TraderConfigData>({
    trader_name: '',
    ai_model: '',
    exchange_id: '',
    btc_eth_leverage: 5,
    altcoin_leverage: 3,
    trading_symbols: '',
    custom_prompt: '',
    override_base_prompt: false,
    system_prompt_template: 'default',
    is_cross_margin: true,
    use_coin_pool: false,
    use_oi_top: false,
    initial_balance: 1000,
    scan_interval_minutes: 3,
  });
  const [isSaving, setIsSaving] = useState(false);
  const [availableCoins, setAvailableCoins] = useState<string[]>([]);
  const [selectedCoins, setSelectedCoins] = useState<string[]>([]);
  const [showCoinSelector, setShowCoinSelector] = useState(false);
  const [promptTemplates, setPromptTemplates] = useState<{name: string}[]>([]);

  useEffect(() => {
    if (traderData) {
      setFormData(traderData);
      // 设置已选择的币种
      if (traderData.trading_symbols) {
        const coins = traderData.trading_symbols.split(',').map(s => s.trim()).filter(s => s);
        setSelectedCoins(coins);
      }
    } else if (!isEditMode) {
      setFormData({
        trader_name: '',
        ai_model: availableModels[0]?.id || '',
        exchange_id: availableExchanges[0]?.id || '',
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: '',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: 'default',
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        initial_balance: 1000,
        scan_interval_minutes: 3,
      });
    }
    // 确保旧数据也有默认的 system_prompt_template
    // 修复BUG：只处理 undefined，不覆盖已有值（包括空字符串）
    if (traderData && traderData.system_prompt_template === undefined) {
      setFormData(prev => ({
        ...prev,
        system_prompt_template: 'default'
      }));
    }
  }, [traderData, isEditMode, availableModels, availableExchanges]);

  // 获取系统配置中的币种列表
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await fetch('/api/config');
        const config = await response.json();
        if (config.default_coins) {
          setAvailableCoins(config.default_coins);
        }
      } catch (error) {
        console.error('Failed to fetch config:', error);
        // 使用默认币种列表
        setAvailableCoins(['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT', 'ADAUSDT']);
      }
    };
    fetchConfig();
  }, []);

  // 获取系统提示词模板列表（修复 #643：每次打开 Modal 时刷新）
  useEffect(() => {
    if (!isOpen) return; // Modal 未打开时不执行

    const fetchPromptTemplates = async () => {
      try {
        const response = await fetch('/api/prompt-templates');
        const data = await response.json();
        if (data.templates) {
          setPromptTemplates(data.templates);
        }
      } catch (error) {
        console.error('Failed to fetch prompt templates:', error);
        // 使用默认模板列表
        setPromptTemplates([{name: 'default'}, {name: 'aggressive'}]);
      }
    };
    fetchPromptTemplates();
  }, [isOpen]); // 依赖 isOpen，每次打开 Modal 时都会重新获取

  if (!isOpen) return null;

  const handleInputChange = (field: keyof TraderConfigData, value: any) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    
    // 如果是直接编辑trading_symbols，同步更新selectedCoins
    if (field === 'trading_symbols') {
      const coins = value.split(',').map((s: string) => s.trim()).filter((s: string) => s);
      setSelectedCoins(coins);
    }
  };

  const handleCoinToggle = (coin: string) => {
    setSelectedCoins(prev => {
      const newCoins = prev.includes(coin)
        ? prev.filter(c => c !== coin)
        : [...prev, coin];

      // 同時更新 formData.trading_symbols
      const symbolsString = newCoins.join(',');
      setFormData(current => ({ ...current, trading_symbols: symbolsString }));

      return newCoins;
    });
  };

  const handleSave = async () => {
    if (!onSave) return;

    setIsSaving(true);
    try {
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        btc_eth_leverage: formData.btc_eth_leverage,
        altcoin_leverage: formData.altcoin_leverage,
        trading_symbols: formData.trading_symbols,
        custom_prompt: formData.custom_prompt,
        override_base_prompt: formData.override_base_prompt,
        system_prompt_template: formData.system_prompt_template,
        is_cross_margin: formData.is_cross_margin,
        use_coin_pool: formData.use_coin_pool,
        use_oi_top: formData.use_oi_top,
        initial_balance: formData.initial_balance,
        scan_interval_minutes: formData.scan_interval_minutes,
      };
      await onSave(saveData);
      onClose();
    } catch (error) {
      console.error('保存失败:', error);
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 backdrop-blur-sm">
      <div 
        className="bg-[#1E2329] border border-[#2B3139] rounded-xl shadow-2xl max-w-3xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35]">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#F0B90B] to-[#E1A706] flex items-center justify-center">
              <span className="text-lg">{isEditMode ? '✏️' : '➕'}</span>
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {isEditMode ? '修改交易员' : '创建交易员'}
              </h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {isEditMode ? '修改交易员配置参数' : '配置新的AI交易员'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-8">
          {/* Basic Info */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              🤖 基础配置
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">交易员名称</label>
                <input
                  type="text"
                  value={formData.trader_name}
                  onChange={(e) => handleInputChange('trader_name', e.target.value)}
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                  placeholder="请输入交易员名称"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">AI模型</label>
                  <select
                    value={formData.ai_model}
                    onChange={(e) => handleInputChange('ai_model', e.target.value)}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                  >
                    {availableModels.map(model => (
                      <option key={model.id} value={model.id}>
                        {getShortName(model.name || model.id).toUpperCase()}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">交易所</label>
                  <select
                    value={formData.exchange_id}
                    onChange={(e) => handleInputChange('exchange_id', e.target.value)}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                  >
                    {availableExchanges.map(exchange => (
                      <option key={exchange.id} value={exchange.id}>
                        {getShortName(exchange.name || exchange.id).toUpperCase()}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* Trading Configuration */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              ⚖️ 交易配置
            </h3>
            <div className="space-y-4">
              {/* 第一行：保证金模式和初始余额 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">保证金模式</label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', true)}
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        formData.is_cross_margin 
                          ? 'bg-[#F0B90B] text-black' 
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
                    >
                      全仓
                    </button>
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', false)}
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        !formData.is_cross_margin 
                          ? 'bg-[#F0B90B] text-black' 
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
                    >
                      逐仓
                    </button>
                  </div>
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">初始余额 ($)</label>
                  <input
                    type="number"
                    value={formData.initial_balance}
                    onChange={(e) => {
                      const value = Number(e.target.value);
                      handleInputChange('initial_balance', value);
                    }}
                    onBlur={(e) => {
                      // 失焦時強制最小值，統一箭頭調整和手動輸入的行為
                      const value = Number(e.target.value);
                      if (value < 100) {
                        handleInputChange('initial_balance', 100);
                      }
                    }}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                    min="100"
                    step="100"
                  />
                  <p className="text-xs text-[#848E9C] mt-1">
                    最小金額 100 USDT（與步進值一致，輸入低於 100 將自動調整）
                  </p>
                </div>
              </div>

              {/* 第二行：AI 扫描决策间隔 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">AI 扫描决策间隔 (分钟)</label>
                  <input
                    type="number"
                    value={formData.scan_interval_minutes}
                    onChange={(e) => handleInputChange('scan_interval_minutes', Number(e.target.value))}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                    min="1"
                    max="60"
                    step="1"
                  />
                  <p className="text-xs text-gray-500 mt-1">建议: 3-10分钟</p>
                </div>
                <div></div>
              </div>

              {/* 第三行：杠杆设置 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">BTC/ETH 杠杆</label>
                  <input
                    type="number"
                    value={formData.btc_eth_leverage}
                    onChange={(e) => handleInputChange('btc_eth_leverage', Number(e.target.value))}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                    min="1"
                    max="125"
                  />
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">山寨币杠杆</label>
                  <input
                    type="number"
                    value={formData.altcoin_leverage}
                    onChange={(e) => handleInputChange('altcoin_leverage', Number(e.target.value))}
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                    min="1"
                    max="75"
                  />
                </div>
              </div>

              {/* 第三行：交易币种 */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-[#EAECEF]">交易币种 (用逗号分隔，留空使用默认)</label>
                  <button
                    type="button"
                    onClick={() => setShowCoinSelector(!showCoinSelector)}
                    className="px-3 py-1 text-xs bg-[#F0B90B] text-black rounded hover:bg-[#E1A706] transition-colors"
                  >
                    {showCoinSelector ? '收起选择' : '快速选择'}
                  </button>
                </div>
                <input
                  type="text"
                  value={formData.trading_symbols}
                  onChange={(e) => handleInputChange('trading_symbols', e.target.value)}
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                  placeholder="例如: BTCUSDT,ETHUSDT,ADAUSDT"
                />
                
                {/* 币种选择器 */}
                {showCoinSelector && (
                  <div className="mt-3 p-3 bg-[#0B0E11] border border-[#2B3139] rounded">
                    <div className="text-xs text-[#848E9C] mb-2">点击选择币种：</div>
                    <div className="flex flex-wrap gap-2">
                      {availableCoins.map(coin => (
                        <button
                          key={coin}
                          type="button"
                          onClick={() => handleCoinToggle(coin)}
                          className={`px-2 py-1 text-xs rounded transition-colors ${
                            selectedCoins.includes(coin)
                              ? 'bg-[#F0B90B] text-black'
                              : 'bg-[#1E2329] text-[#848E9C] border border-[#2B3139] hover:border-[#F0B90B]'
                          }`}
                        >
                          {coin.replace('USDT', '')}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {/* 幣種數量提示 */}
                {selectedCoins.length > 0 && (
                  <div className="mt-3 space-y-2">
                    {/* 當前數量顯示 */}
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-[#848E9C]">
                        已選擇 <span className={`font-semibold ${
                          selectedCoins.length > MAX_SYMBOLS ? 'text-[#F6465D]' :
                          selectedCoins.length > WARNING_THRESHOLD ? 'text-[#F0B90B]' :
                          'text-[#02C076]'
                        }`}>{selectedCoins.length}</span> 個幣種
                      </span>
                      <span className="text-[#848E9C]">上限：{MAX_SYMBOLS} 個</span>
                    </div>

                    {/* 進度條 */}
                    <div className="w-full h-1 bg-[#2B3139] rounded-full overflow-hidden">
                      <div
                        className={`h-full transition-all duration-300 ${
                          selectedCoins.length > MAX_SYMBOLS ? 'bg-[#F6465D]' :
                          selectedCoins.length > WARNING_THRESHOLD ? 'bg-[#F0B90B]' :
                          'bg-[#02C076]'
                        }`}
                        style={{ width: `${Math.min((selectedCoins.length / MAX_SYMBOLS) * 100, 100)}%` }}
                      />
                    </div>

                    {/* 警告提示 */}
                    {selectedCoins.length > WARNING_THRESHOLD && selectedCoins.length <= MAX_SYMBOLS && (
                      <div className="flex items-start gap-2 p-3 bg-[#2B1F0B] border border-[#F0B90B]/30 rounded text-xs">
                        <svg className="w-4 h-4 text-[#F0B90B] flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                        </svg>
                        <div className="flex-1">
                          <div className="text-[#F0B90B] font-semibold mb-1">接近幣種數量上限</div>
                          <div className="text-[#848E9C]">
                            當前：{selectedCoins.length} 個幣種 × 4 時間週期 = {selectedCoins.length * 4} 個 WebSocket 流
                            <br />
                            Binance 限制：1024 流/連接
                          </div>
                        </div>
                      </div>
                    )}

                    {/* 錯誤提示 */}
                    {selectedCoins.length > MAX_SYMBOLS && (
                      <div className="flex items-start gap-2 p-3 bg-[#2B1315] border border-[#F6465D]/30 rounded text-xs">
                        <svg className="w-4 h-4 text-[#F6465D] flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <div className="flex-1">
                          <div className="text-[#F6465D] font-semibold mb-1">幣種數量超過上限！</div>
                          <div className="text-[#848E9C]">
                            超過 {MAX_SYMBOLS} 個幣種會導致：
                            <br />
                            • WebSocket 流數超過 Binance 限制（1024 流）
                            <br />
                            • <span className="text-[#F6465D] font-semibold">系統會自動忽略前 {MAX_SYMBOLS} 個之後的幣種</span>
                            <br />
                            • 建議減少幣種數量或使用幣池功能
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Signal Sources */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              📡 信号源配置
            </h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.use_coin_pool}
                  onChange={(e) => handleInputChange('use_coin_pool', e.target.checked)}
                  className="w-4 h-4"
                />
                <label className="text-sm text-[#EAECEF]">使用 Coin Pool 信号</label>
              </div>
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.use_oi_top}
                  onChange={(e) => handleInputChange('use_oi_top', e.target.checked)}
                  className="w-4 h-4"
                />
                <label className="text-sm text-[#EAECEF]">使用 OI Top 信号</label>
              </div>
            </div>
          </div>

          {/* Trading Prompt */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              💬 交易策略提示词
            </h3>
            <div className="space-y-4">
              {/* 系统提示词模板选择 */}
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">{t('systemPromptTemplate', language)}</label>
                <select
                  value={formData.system_prompt_template}
                  onChange={(e) => handleInputChange('system_prompt_template', e.target.value)}
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                >
                  {promptTemplates.map(template => {
                    // Template name mapping with i18n
                    const getTemplateName = (name: string) => {
                      const keyMap: Record<string, string> = {
                        'default': 'promptTemplateDefault',
                        'adaptive': 'promptTemplateAdaptive',
                        'adaptive_moderate': 'promptTemplateAdaptiveModerate',
                        'adaptive_relaxed': 'promptTemplateAdaptiveRelaxed',
                        'adaptive_altcoin': 'promptTemplateAdaptiveAltcoin',
                        'Hansen': 'promptTemplateHansen',
                        'nof1': 'promptTemplateNof1',
                        'taro_long_prompts': 'promptTemplateTaroLong',
                      };
                      const key = keyMap[name];
                      return key ? t(key, language) : name.charAt(0).toUpperCase() + name.slice(1);
                    };

                    return (
                      <option key={template.name} value={template.name}>
                        {getTemplateName(template.name)}
                      </option>
                    );
                  })}
                </select>

                {/* 動態描述區域 */}
                <div className="mt-2 p-3 rounded" style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.15)' }}>
                  <div className="text-xs font-semibold mb-1" style={{ color: '#F0B90B' }}>
                    {(() => {
                      const titleKeyMap: Record<string, string> = {
                        'default': 'promptDescDefault',
                        'adaptive': 'promptDescAdaptive',
                        'adaptive_moderate': 'promptDescAdaptiveModerate',
                        'adaptive_relaxed': 'promptDescAdaptiveRelaxed',
                        'adaptive_altcoin': 'promptDescAdaptiveAltcoin',
                        'Hansen': 'promptDescHansen',
                        'nof1': 'promptDescNof1',
                        'taro_long_prompts': 'promptDescTaroLong',
                      };
                      const key = titleKeyMap[formData.system_prompt_template];
                      return key ? t(key, language) : t('promptDescDefault', language);
                    })()}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {(() => {
                      const contentKeyMap: Record<string, string> = {
                        'default': 'promptDescDefaultContent',
                        'adaptive': 'promptDescAdaptiveContent',
                        'adaptive_moderate': 'promptDescAdaptiveModerateContent',
                        'adaptive_relaxed': 'promptDescAdaptiveRelaxedContent',
                        'adaptive_altcoin': 'promptDescAdaptiveAltcoinContent',
                        'Hansen': 'promptDescHansenContent',
                        'nof1': 'promptDescNof1Content',
                        'taro_long_prompts': 'promptDescTaroLongContent',
                      };
                      const key = contentKeyMap[formData.system_prompt_template];
                      return key ? t(key, language) : t('promptDescDefaultContent', language);
                    })()}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.override_base_prompt}
                  onChange={(e) => handleInputChange('override_base_prompt', e.target.checked)}
                  className="w-4 h-4"
                />
                <label className="text-sm text-[#EAECEF]">覆盖默认提示词</label>
                <span className="text-xs text-[#F0B90B] inline-flex items-center gap-1"><svg xmlns="http://www.w3.org/2000/svg" className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg> 启用后将完全替换默认策略</span>
              </div>
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {formData.override_base_prompt ? '自定义提示词' : '附加提示词'}
                </label>
                <textarea
                  value={formData.custom_prompt}
                  onChange={(e) => handleInputChange('custom_prompt', e.target.value)}
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none h-24 resize-none"
                  placeholder={formData.override_base_prompt ? "输入完整的交易策略提示词..." : "输入额外的交易策略提示..."}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35]">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-all duration-200 border border-[#404750]"
          >
            取消
          </button>
          {onSave && (
            <button
              onClick={handleSave}
              disabled={isSaving || !formData.trader_name || !formData.ai_model || !formData.exchange_id}
              className="px-8 py-3 bg-gradient-to-r from-[#F0B90B] to-[#E1A706] text-black rounded-lg hover:from-[#E1A706] hover:to-[#D4951E] transition-all duration-200 disabled:bg-[#848E9C] disabled:cursor-not-allowed font-medium shadow-lg"
            >
              {isSaving ? '保存中...' : (isEditMode ? '保存修改' : '创建交易员')}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
