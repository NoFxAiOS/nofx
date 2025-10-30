import React, { useState, useEffect } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import type { TraderInfo, CreateTraderRequest, AIModel, Exchange } from '../types';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';

export function AITradersPage() {
  const { language } = useLanguage();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showModelModal, setShowModelModal] = useState(false);
  const [showExchangeModal, setShowExchangeModal] = useState(false);
  const [editingModel, setEditingModel] = useState<string | null>(null);
  const [editingExchange, setEditingExchange] = useState<string | null>(null);
  const [allModels, setAllModels] = useState<AIModel[]>([]);
  const [allExchanges, setAllExchanges] = useState<Exchange[]>([]);

  const { data: traders, mutate: mutateTraders } = useSWR<TraderInfo[]>(
    'traders',
    api.getTraders,
    { refreshInterval: 5000 }
  );

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      try {
        const [modelConfigs, exchangeConfigs] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs()
        ]);
        setAllModels(modelConfigs);
        setAllExchanges(exchangeConfigs);
      } catch (error) {
        console.error('Failed to load configs:', error);
      }
    };
    loadConfigs();
  }, []);

  // 只显示已配置的模型和交易所
  const configuredModels = allModels.filter(m => m.enabled && m.apiKey);
  const configuredExchanges = allExchanges.filter(e => e.enabled && e.apiKey && (e.id === 'hyperliquid' || e.secretKey));

  // 检查模型是否正在被运行中的交易员使用
  const isModelInUse = (modelId: string) => {
    return traders?.some(t => t.ai_model === modelId && t.is_running) || false;
  };

  // 检查交易所是否正在被运行中的交易员使用
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some(t => t.exchange_id === exchangeId && t.is_running) || false;
  };

  const handleCreateTrader = async (modelId: string, exchangeId: string, name: string, initialBalance: number) => {
    try {
      const model = allModels.find(m => m.id === modelId);
      const exchange = allExchanges.find(e => e.id === exchangeId);
      
      if (!model?.enabled) {
        alert(t('modelNotConfigured', language));
        return;
      }
      
      if (!exchange?.enabled) {
        alert(t('exchangeNotConfigured', language));
        return;
      }
      
      const request: CreateTraderRequest = {
        name,
        ai_model_id: modelId,
        exchange_id: exchangeId,
        initial_balance: initialBalance
      };
      
      await api.createTrader(request);
      setShowCreateModal(false);
      mutateTraders();
    } catch (error) {
      console.error('Failed to create trader:', error);
      alert('创建交易员失败');
    }
  };

  const handleDeleteTrader = async (traderId: string) => {
    if (!confirm(t('confirmDeleteTrader', language))) return;
    
    try {
      await api.deleteTrader(traderId);
      mutateTraders();
    } catch (error) {
      console.error('Failed to delete trader:', error);
      alert('删除交易员失败');
    }
  };

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await api.stopTrader(traderId);
      } else {
        await api.startTrader(traderId);
      }
      mutateTraders();
    } catch (error) {
      console.error('Failed to toggle trader:', error);
      alert('操作失败');
    }
  };

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId);
      setShowModelModal(true);
    }
  };

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId);
      setShowExchangeModal(true);
    }
  };

  const handleDeleteModelConfig = async (modelId: string) => {
    if (!confirm('确定要删除此AI模型配置吗？')) return;
    
    try {
      const updatedModels = allModels.map(m => 
        m.id === modelId ? { ...m, apiKey: '', enabled: false } : m
      );
      
      const request = {
        models: Object.fromEntries(
          updatedModels.map(model => [
            model.id,
            {
              enabled: model.enabled,
              api_key: model.apiKey || ''
            }
          ])
        )
      };
      
      await api.updateModelConfigs(request);
      setAllModels(updatedModels);
      setShowModelModal(false);
      setEditingModel(null);
    } catch (error) {
      console.error('Failed to delete model config:', error);
      alert('删除配置失败');
    }
  };

  const handleSaveModelConfig = async (modelId: string, apiKey: string) => {
    try {
      const updatedModels = allModels.map(m => 
        m.id === modelId ? { ...m, apiKey, enabled: true } : m
      );
      
      const request = {
        models: Object.fromEntries(
          updatedModels.map(model => [
            model.id,
            {
              enabled: model.enabled,
              api_key: model.apiKey || ''
            }
          ])
        )
      };
      
      await api.updateModelConfigs(request);
      setAllModels(updatedModels);
      setShowModelModal(false);
      setEditingModel(null);
    } catch (error) {
      console.error('Failed to save model config:', error);
      alert('保存配置失败');
    }
  };

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    if (!confirm('确定要删除此交易所配置吗？')) return;
    
    try {
      const updatedExchanges = allExchanges.map(e => 
        e.id === exchangeId ? { ...e, apiKey: '', secretKey: '', enabled: false } : e
      );
      
      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map(exchange => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false
            }
          ])
        )
      };
      
      await api.updateExchangeConfigs(request);
      setAllExchanges(updatedExchanges);
      setShowExchangeModal(false);
      setEditingExchange(null);
    } catch (error) {
      console.error('Failed to delete exchange config:', error);
      alert('删除交易所配置失败');
    }
  };

  const handleSaveExchangeConfig = async (exchangeId: string, apiKey: string, secretKey?: string, testnet?: boolean) => {
    try {
      const updatedExchanges = allExchanges.map(e => 
        e.id === exchangeId ? { ...e, apiKey, secretKey, testnet, enabled: true } : e
      );
      
      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map(exchange => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false
            }
          ])
        )
      };
      
      await api.updateExchangeConfigs(request);
      setAllExchanges(updatedExchanges);
      setShowExchangeModal(false);
      setEditingExchange(null);
    } catch (error) {
      console.error('Failed to save exchange config:', error);
      alert('保存交易所配置失败');
    }
  };

  const handleAddModel = () => {
    setEditingModel(null);
    setShowModelModal(true);
  };

  const handleAddExchange = () => {
    setEditingExchange(null);
    setShowExchangeModal(true);
  };

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl" style={{
            background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
            boxShadow: '0 4px 14px rgba(240, 185, 11, 0.4)'
          }}>
            🤖
          </div>
          <div>
            <h1 className="text-2xl font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
              {t('aiTraders', language)}
              <span className="text-xs font-normal px-2 py-1 rounded" style={{ 
                background: 'rgba(240, 185, 11, 0.15)', 
                color: '#F0B90B' 
              }}>
                {traders?.length || 0} {t('active', language)}
              </span>
            </h1>
            <p className="text-xs" style={{ color: '#848E9C' }}>
              {t('manageAITraders', language)}
            </p>
          </div>
        </div>
        
        <div className="flex gap-3">
          <button
            onClick={handleAddModel}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{ 
              background: '#2B3139', 
              color: '#EAECEF', 
              border: '1px solid #474D57' 
            }}
          >
            ➕ {t('aiModels', language)}
          </button>
          
          <button
            onClick={handleAddExchange}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{ 
              background: '#2B3139', 
              color: '#EAECEF', 
              border: '1px solid #474D57' 
            }}
          >
            ➕ {t('exchanges', language)}
          </button>
          
          <button
            onClick={() => setShowCreateModal(true)}
            disabled={configuredModels.length === 0 || configuredExchanges.length === 0}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ 
              background: (configuredModels.length > 0 && configuredExchanges.length > 0) ? '#F0B90B' : '#2B3139', 
              color: (configuredModels.length > 0 && configuredExchanges.length > 0) ? '#000' : '#848E9C' 
            }}
          >
            ➕ {t('createTrader', language)}
          </button>
        </div>
      </div>

      {/* Configuration Status */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* AI Models */}
        <div className="binance-card p-4">
          <h3 className="text-lg font-semibold mb-3" style={{ color: '#EAECEF' }}>
            🧠 {t('aiModels', language)}
          </h3>
          <div className="space-y-3">
            {configuredModels.map(model => {
              const inUse = isModelInUse(model.id);
              return (
                <div 
                  key={model.id} 
                  className={`flex items-center justify-between p-3 rounded transition-all ${
                    inUse ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleModelClick(model.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                         style={{ 
                           background: model.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                           color: '#fff'
                         }}>
                      {model.name[0]}
                    </div>
                    <div>
                      <div className="font-semibold" style={{ color: '#EAECEF' }}>{model.name}</div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div className={`w-3 h-3 rounded-full bg-green-400`} />
                </div>
              );
            })}
            {configuredModels.length === 0 && (
              <div className="text-center py-8" style={{ color: '#848E9C' }}>
                <div className="text-2xl mb-2">🧠</div>
                <div className="text-sm">暂无已配置的AI模型</div>
              </div>
            )}
          </div>
        </div>

        {/* Exchanges */}
        <div className="binance-card p-4">
          <h3 className="text-lg font-semibold mb-3" style={{ color: '#EAECEF' }}>
            🏦 {t('exchanges', language)}
          </h3>
          <div className="space-y-3">
            {configuredExchanges.map(exchange => {
              const inUse = isExchangeInUse(exchange.id);
              return (
                <div 
                  key={exchange.id} 
                  className={`flex items-center justify-between p-3 rounded transition-all ${
                    inUse ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleExchangeClick(exchange.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                         style={{ 
                           background: exchange.type === 'cex' ? '#F0B90B' : '#0ECB81',
                           color: '#000'
                         }}>
                      {exchange.name[0]}
                    </div>
                    <div>
                      <div className="font-semibold" style={{ color: '#EAECEF' }}>{exchange.name}</div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {exchange.type.toUpperCase()} • {t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div className={`w-3 h-3 rounded-full bg-green-400`} />
                </div>
              );
            })}
            {configuredExchanges.length === 0 && (
              <div className="text-center py-8" style={{ color: '#848E9C' }}>
                <div className="text-2xl mb-2">🏦</div>
                <div className="text-sm">暂无已配置的交易所</div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Traders List */}
      <div className="binance-card p-6">
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-xl font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
            👥 {t('currentTraders', language)}
          </h2>
        </div>

        {traders && traders.length > 0 ? (
          <div className="space-y-4">
            {traders.map(trader => (
              <div key={trader.trader_id} 
                   className="flex items-center justify-between p-4 rounded transition-all hover:translate-y-[-1px]"
                   style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-full flex items-center justify-center text-xl"
                       style={{ 
                         background: trader.ai_model === 'deepseek' ? '#60a5fa' : '#c084fc',
                         color: '#fff'
                       }}>
                    🤖
                  </div>
                  <div>
                    <div className="font-bold text-lg" style={{ color: '#EAECEF' }}>
                      {trader.trader_name}
                    </div>
                    <div className="text-sm" style={{ 
                      color: trader.ai_model === 'deepseek' ? '#60a5fa' : '#c084fc' 
                    }}>
                      {trader.ai_model.toUpperCase()} Model • {trader.exchange_id?.toUpperCase()}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  {/* Status */}
                  <div className="text-center">
                    <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('status', language)}</div>
                    <div className={`px-3 py-1 rounded text-xs font-bold ${
                      trader.is_running ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                    }`} style={trader.is_running 
                      ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                      : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                    }>
                      {trader.is_running ? t('running', language) : t('stopped', language)}
                    </div>
                  </div>

                  {/* Actions */}
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleToggleTrader(trader.trader_id, trader.is_running)}
                      className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                      style={trader.is_running 
                        ? { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                        : { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                      }
                    >
                      {trader.is_running ? t('stop', language) : t('start', language)}
                    </button>
                    
                    <button
                      onClick={() => handleDeleteTrader(trader.trader_id)}
                      className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
                    >
                      🗑️
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-16" style={{ color: '#848E9C' }}>
            <div className="text-6xl mb-4 opacity-50">🤖</div>
            <div className="text-lg font-semibold mb-2">{t('noTraders', language)}</div>
            <div className="text-sm mb-4">{t('createFirstTrader', language)}</div>
            {(configuredModels.length === 0 || configuredExchanges.length === 0) && (
              <div className="text-sm text-yellow-500">
                {configuredModels.length === 0 && configuredExchanges.length === 0 
                  ? t('configureModelsAndExchangesFirst', language)
                  : configuredModels.length === 0 
                    ? t('configureModelsFirst', language)
                    : t('configureExchangesFirst', language)
                }
              </div>
            )}
          </div>
        )}
      </div>

      {/* Create Trader Modal */}
      {showCreateModal && (
        <CreateTraderModal
          enabledModels={configuredModels}
          enabledExchanges={configuredExchanges}
          onCreate={handleCreateTrader}
          onClose={() => setShowCreateModal(false)}
          language={language}
        />
      )}

      {/* Model Configuration Modal */}
      {showModelModal && (
        <ModelConfigModal
          allModels={allModels}
          editingModelId={editingModel}
          onSave={handleSaveModelConfig}
          onDelete={handleDeleteModelConfig}
          onClose={() => {
            setShowModelModal(false);
            setEditingModel(null);
          }}
          language={language}
        />
      )}

      {/* Exchange Configuration Modal */}
      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={allExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchangeConfig}
          onDelete={handleDeleteExchangeConfig}
          onClose={() => {
            setShowExchangeModal(false);
            setEditingExchange(null);
          }}
          language={language}
        />
      )}
    </div>
  );
}

// Create Trader Modal Component
function CreateTraderModal({ 
  enabledModels, 
  enabledExchanges,
  onCreate, 
  onClose, 
  language 
}: {
  enabledModels: AIModel[];
  enabledExchanges: Exchange[];
  onCreate: (modelId: string, exchangeId: string, name: string, initialBalance: number) => void;
  onClose: () => void;
  language: any;
}) {
  // 默认选择DeepSeek模型，如果没有启用则选择第一个
  const defaultModel = enabledModels.find(m => m.id === 'deepseek') || enabledModels[0];
  // 默认选择Binance交易所，如果没有启用则选择第一个
  const defaultExchange = enabledExchanges.find(e => e.id === 'binance') || enabledExchanges[0];
  
  const [selectedModel, setSelectedModel] = useState(defaultModel?.id || '');
  const [selectedExchange, setSelectedExchange] = useState(defaultExchange?.id || '');
  const [traderName, setTraderName] = useState('');
  const [initialBalance, setInitialBalance] = useState(1000);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedModel || !selectedExchange || !traderName.trim()) return;
    
    onCreate(selectedModel, selectedExchange, traderName.trim(), initialBalance);
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" style={{ background: '#1E2329' }}>
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('createNewTrader', language)}
        </h3>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('selectAIModel', language)}
            </label>
            <select
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            >
              {enabledModels.map(model => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('traderName', language)}
            </label>
            <input
              type="text"
              value={traderName}
              onChange={(e) => setTraderName(e.target.value)}
              placeholder={t('enterTraderName', language)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            />
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('selectExchange', language)}
            </label>
            <select
              value={selectedExchange}
              onChange={(e) => setSelectedExchange(e.target.value)}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            >
              {enabledExchanges.map(exchange => (
                <option key={exchange.id} value={exchange.id}>
                  {exchange.name} ({exchange.type.toUpperCase()})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              初始资金 (USDT)
            </label>
            <input
              type="number"
              value={initialBalance}
              onChange={(e) => setInitialBalance(Number(e.target.value))}
              min="100"
              max="100000"
              className="w-full px-3 py-2 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
              required
            />
          </div>

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('create', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Model Configuration Modal Component  
function ModelConfigModal({
  allModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language
}: {
  allModels: AIModel[];
  editingModelId: string | null;
  onSave: (modelId: string, apiKey: string) => void;
  onDelete: (modelId: string) => void;
  onClose: () => void;
  language: any;
}) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '');
  const [apiKey, setApiKey] = useState('');

  // 获取当前编辑的模型信息
  const selectedModel = allModels.find(m => m.id === selectedModelId);

  // 如果是编辑现有模型，初始化API Key
  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '');
    }
  }, [editingModelId, selectedModel]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedModelId || !apiKey.trim()) return;
    
    onSave(selectedModelId, apiKey.trim());
  };

  // 可选择的模型列表（排除已配置的，除非是当前编辑的）
  const availableModels = allModels.filter(m => 
    !m.enabled || !m.apiKey || m.id === editingModelId
  );

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg" style={{ background: '#1E2329' }}>
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {editingModelId ? '编辑AI模型' : '添加AI模型'}
        </h3>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingModelId && (
            <div>
              <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                选择AI模型
              </label>
              <select
                value={selectedModelId}
                onChange={(e) => setSelectedModelId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                required
              >
                <option value="">请选择模型</option>
                {availableModels.map(model => (
                  <option key={model.id} value={model.id}>
                    {model.name} ({model.provider})
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedModel && (
            <div className="p-4 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                     style={{ 
                       background: selectedModel.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                       color: '#fff'
                     }}>
                  {selectedModel.name[0]}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>{selectedModel.name}</div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>{selectedModel.provider}</div>
                </div>
              </div>
              
              <div>
                <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                  API Key
                </label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={`请输入 ${selectedModel.name} API Key`}
                  className="w-full px-3 py-2 rounded"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  required
                />
              </div>
            </div>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            {editingModelId && (
              <button
                type="button"
                onClick={() => {
                  onDelete(editingModelId);
                }}
                className="px-4 py-2 rounded text-sm font-semibold"
                style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              >
                🗑️
              </button>
            )}
            <button
              type="submit"
              disabled={!selectedModelId || !apiKey.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Exchange Configuration Modal Component
function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language
}: {
  allExchanges: Exchange[];
  editingExchangeId: string | null;
  onSave: (exchangeId: string, apiKey: string, secretKey?: string, testnet?: boolean) => void;
  onDelete: (exchangeId: string) => void;
  onClose: () => void;
  language: any;
}) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(editingExchangeId || '');
  const [apiKey, setApiKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [testnet, setTestnet] = useState(false);

  // 获取当前编辑的交易所信息
  const selectedExchange = allExchanges.find(e => e.id === selectedExchangeId);

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setApiKey(selectedExchange.apiKey || '');
      setSecretKey(selectedExchange.secretKey || '');
      setTestnet(selectedExchange.testnet || false);
    }
  }, [editingExchangeId, selectedExchange]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedExchangeId || !apiKey.trim()) return;
    if (selectedExchange?.id !== 'hyperliquid' && !secretKey.trim()) return;
    
    onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet);
  };

  // 可选择的交易所列表（排除已配置的，除非是当前编辑的）
  const availableExchanges = allExchanges.filter(e => 
    !e.enabled || !e.apiKey || e.id === editingExchangeId
  );

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg" style={{ background: '#1E2329' }}>
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {editingExchangeId ? '编辑交易所' : '添加交易所'}
        </h3>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingExchangeId && (
            <div>
              <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                选择交易所
              </label>
              <select
                value={selectedExchangeId}
                onChange={(e) => setSelectedExchangeId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                required
              >
                <option value="">请选择交易所</option>
                {availableExchanges.map(exchange => (
                  <option key={exchange.id} value={exchange.id}>
                    {exchange.name} ({exchange.type.toUpperCase()})
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedExchange && (
            <div className="p-4 rounded" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                     style={{ 
                       background: selectedExchange.type === 'cex' ? '#F0B90B' : '#0ECB81',
                       color: '#000'
                     }}>
                  {selectedExchange.name[0]}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>{selectedExchange.name}</div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>{selectedExchange.type.toUpperCase()}</div>
                </div>
              </div>
              
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                    {selectedExchange.id === 'hyperliquid' ? 'Private Key (无需0x前缀)' : 'API Key'}
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={selectedExchange.id === 'hyperliquid' ? '请输入以太坊私钥' : `请输入 ${selectedExchange.name} API Key`}
                    className="w-full px-3 py-2 rounded"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    required
                  />
                </div>
                
                {selectedExchange.id !== 'hyperliquid' && (
                  <div>
                    <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                      Secret Key
                    </label>
                    <input
                      type="password"
                      value={secretKey}
                      onChange={(e) => setSecretKey(e.target.value)}
                      placeholder={`请输入 ${selectedExchange.name} Secret Key`}
                      className="w-full px-3 py-2 rounded"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                      required
                    />
                  </div>
                )}

                {selectedExchange.type === 'dex' && (
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={testnet}
                      onChange={(e) => setTestnet(e.target.checked)}
                      className="w-4 h-4"
                    />
                    <label className="text-sm" style={{ color: '#EAECEF' }}>
                      {t('useTestnet', language)}
                    </label>
                  </div>
                )}
              </div>
            </div>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => {
                  onDelete(editingExchangeId);
                }}
                className="px-4 py-2 rounded text-sm font-semibold"
                style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              >
                🗑️
              </button>
            )}
            <button
              type="submit"
              disabled={!selectedExchangeId || !apiKey.trim() || (selectedExchange?.id !== 'hyperliquid' && !secretKey.trim())}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}