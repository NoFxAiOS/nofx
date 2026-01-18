import { useState, useEffect } from 'react'
import { X as IconX, Bell, Send, Loader2 } from 'lucide-react'
import { httpClient } from '../lib/httpClient'
import { toast } from 'sonner'

interface NotificationConfig {
  id?: string
  user_id?: string
  trader_id?: string
  wx_pusher_token?: string
  wx_pusher_uids?: string
  is_enabled: boolean
  enable_decision?: boolean
  enable_trade_open?: boolean
  enable_trade_close?: boolean
  created_at?: string
  updated_at?: string
}

interface NotificationConfigModalProps {
  isOpen: boolean
  onClose: () => void
  traderId: string
}

export function NotificationConfigModal({
  isOpen,
  onClose,
  traderId,
}: NotificationConfigModalProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [testingScenario, setTestingScenario] = useState<string | null>(null)
  const [config, setConfig] = useState<NotificationConfig>({
    is_enabled: false,
    wx_pusher_token: '',
    wx_pusher_uids: '',
    enable_decision: true,
    enable_trade_open: true,
    enable_trade_close: true,
  })

  // 加载现有配置
  useEffect(() => {
    if (isOpen && traderId) {
      loadConfig()
    }
  }, [isOpen, traderId])

  const loadConfig = async () => {
    setIsLoading(true)
    try {
      const result = await httpClient.get<NotificationConfig>(
        `/api/notifications/config?trader_id=${traderId}`
      )
      if (result.success && result.data) {
        const data = result.data
        setConfig({
          ...data,
          enable_decision: data.enable_decision ?? true,
          enable_trade_open: data.enable_trade_open ?? true,
          enable_trade_close: data.enable_trade_close ?? true,
        })
      }
    } catch (error) {
      console.error('Failed to load notification config:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSave = async () => {
    // 验证必填字段
    if (config.is_enabled) {
      if (!config.wx_pusher_token || config.wx_pusher_token === '***') {
        toast.error('请输入 WxPusher Token')
        return
      }
      if (!config.wx_pusher_uids) {
        toast.error('请输入微信用户 ID')
        return
      }
      // 验证 UIDs 格式
      try {
        const uids = JSON.parse(config.wx_pusher_uids)
        if (!Array.isArray(uids) || uids.length === 0) {
          toast.error('微信用户 ID 必须是非空数组')
          return
        }
      } catch (e) {
        toast.error('微信用户 ID 格式错误,必须是 JSON 数组')
        return
      }
    }

    setIsSaving(true)
    try {
      const result = await httpClient.post(
        `/api/notifications/config?trader_id=${traderId}`,
        {
          is_enabled: config.is_enabled,
          wx_pusher_token: config.wx_pusher_token === '***' ? undefined : config.wx_pusher_token,
          wx_pusher_uids: config.wx_pusher_uids,
          enable_decision: config.enable_decision,
          enable_trade_open: config.enable_trade_open,
          enable_trade_close: config.enable_trade_close,
        }
      )
      if (result.success) {
        toast.success('通知配置已保存')
        onClose()
      } else {
        toast.error(result.message || '保存失败')
      }
    } catch (error: any) {
      toast.error(error.message || '保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  const handleTest = async (scenario: 'decision' | 'trade_open' | 'trade_close') => {
    setTestingScenario(scenario)
    try {
      const result = await httpClient.post(
        `/api/notifications/test?trader_id=${traderId}&type=${scenario}`,
        {}
      )
      if (result.success) {
        toast.success('测试通知已发送,请检查微信')
      } else {
        toast.error(result.message || '发送失败')
      }
    } catch (error: any) {
      toast.error(error.message || '发送失败')
    } finally {
      setTestingScenario(null)
    }
  }

  const handleDisable = async () => {
    setIsSaving(true)
    try {
      const result = await httpClient.delete(
        `/api/notifications/config?trader_id=${traderId}`
      )
      if (result.success) {
        toast.success('通知已禁用')
        setConfig({ is_enabled: false, wx_pusher_token: '', wx_pusher_uids: '' })
      } else {
        toast.error(result.message || '禁用失败')
      }
    } catch (error: any) {
      toast.error(error.message || '禁用失败')
    } finally {
      setIsSaving(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
      <div className="bg-[#1E2329] rounded-xl w-full max-w-2xl shadow-2xl border border-[#2B3139]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#2B3139]">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#F0B90B] to-[#E1A706] flex items-center justify-center">
              <Bell className="w-5 h-5 text-black" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">微信通知配置</h2>
              <p className="text-sm text-[#848E9C] mt-1">
                配置 WxPusher 接收 AI 交易决策通知
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors"
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-6 h-6 animate-spin text-[#F0B90B]" />
            </div>
          ) : (
            <>
              {/* Enable/Disable */}
              <div className="flex items-center justify-between p-4 bg-[#0B0E11] rounded-lg border border-[#2B3139]">
                <div>
                  <label className="text-sm font-medium text-[#EAECEF]">
                    启用微信通知
                  </label>
                  <p className="text-xs text-[#848E9C] mt-1">
                    开启后将通过 WxPusher 推送交易通知到微信
                  </p>
                </div>
                <button
                  onClick={() =>
                    setConfig({ ...config, is_enabled: !config.is_enabled })
                  }
                  className={`relative w-12 h-6 rounded-full transition-colors ${
                    config.is_enabled ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
                  }`}
                >
                  <div
                    className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-transform ${
                      config.is_enabled ? 'translate-x-7' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* WxPusher Token */}
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  WxPusher Token <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={config.wx_pusher_token || ''}
                  onChange={(e) =>
                    setConfig({ ...config, wx_pusher_token: e.target.value })
                  }
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none font-mono text-sm"
                  placeholder="AT_xxxxxxxxxxxxxxxx"
                  disabled={!config.is_enabled}
                />
                <p className="text-xs text-[#848E9C] mt-2">
                  从 <a href="https://wxpusher.zjiecode.com/admin/main/app/appToken" target="_blank" rel="noopener noreferrer" className="text-[#F0B90B] hover:underline">WxPusher 管理后台</a> 获取 appToken
                </p>
              </div>

              {/* WxPusher UIDs */}
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  微信用户 ID <span className="text-red-500">*</span>
                </label>
                <textarea
                  value={config.wx_pusher_uids || ''}
                  onChange={(e) =>
                    setConfig({ ...config, wx_pusher_uids: e.target.value })
                  }
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none font-mono text-sm h-24 resize-none"
                  placeholder='["UID_xxxxxxxxx", "UID_yyyyyyyyy"]'
                  disabled={!config.is_enabled}
                />
                <p className="text-xs text-[#848E9C] mt-2">
                  格式为 JSON 数组,从 <a href="https://wxpusher.zjiecode.com/admin/main/app/user" target="_blank" rel="noopener noreferrer" className="text-[#F0B90B] hover:underline">用户管理</a> 获取 UID
                </p>
              </div>

              {/* Scenario toggles & tests */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                {[
                  {
                    key: 'decision',
                    title: 'AI 决策',
                    enabled: config.enable_decision,
                    onToggle: () =>
                      setConfig((prev) => ({ ...prev, enable_decision: !prev.enable_decision })),
                  },
                  {
                    key: 'trade_open',
                    title: '开仓通知',
                    enabled: config.enable_trade_open,
                    onToggle: () =>
                      setConfig((prev) => ({ ...prev, enable_trade_open: !prev.enable_trade_open })),
                  },
                  {
                    key: 'trade_close',
                    title: '平仓通知',
                    enabled: config.enable_trade_close,
                    onToggle: () =>
                      setConfig((prev) => ({ ...prev, enable_trade_close: !prev.enable_trade_close })),
                  },
                ].map((item) => (
                  <div key={item.key} className="p-4 bg-[#0B0E11] border border-[#2B3139] rounded-lg space-y-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <div className="text-sm font-medium text-[#EAECEF]">{item.title}</div>
                        <div className="text-xs text-[#848E9C] mt-1">开启后才会推送该场景的通知</div>
                      </div>
                      <button
                        onClick={item.onToggle}
                        disabled={!config.is_enabled}
                        className={`relative w-12 h-6 rounded-full transition-colors ${
                          item.enabled ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
                        } ${!config.is_enabled ? 'opacity-60 cursor-not-allowed' : ''}`}
                      >
                        <div
                          className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-transform ${
                            item.enabled ? 'translate-x-7' : 'translate-x-1'
                          }`}
                        />
                      </button>
                    </div>
                    <button
                      onClick={() => handleTest(item.key as 'decision' | 'trade_open' | 'trade_close')}
                      disabled={
                        !config.is_enabled ||
                        !item.enabled ||
                        !config.wx_pusher_token ||
                        !config.wx_pusher_uids ||
                        testingScenario === item.key
                      }
                      className="w-full px-3 py-2 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                    >
                      {testingScenario === item.key ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          测试发送中...
                        </>
                      ) : (
                        <>
                          <Send className="w-4 h-4" />
                          发送测试
                        </>
                      )}
                    </button>
                  </div>
                ))}
              </div>

              {/* Info Box */}
              <div className="p-4 bg-[#0B0E11] border border-[#2B3139] rounded-lg">
                <h4 className="text-sm font-medium text-[#EAECEF] mb-2">
                  如何配置 WxPusher?
                </h4>
                <ol className="text-xs text-[#848E9C] space-y-1 list-decimal list-inside">
                  <li>访问 <a href="https://wxpusher.zjiecode.com" target="_blank" rel="noopener noreferrer" className="text-[#F0B90B] hover:underline">WxPusher 官网</a> 注册账号</li>
                  <li>创建应用并获取 appToken</li>
                  <li>使用微信扫码关注您的应用</li>
                  <li>在用户管理中查看并复制 UID</li>
                  <li>将 Token 和 UID 填入上方表单</li>
                </ol>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-between gap-3 p-6 border-t border-[#2B3139]">
          <div className="flex gap-2">
            {config.is_enabled && (
              <button
                onClick={handleDisable}
                disabled={isSaving}
                className="px-4 py-2 bg-red-500/10 text-red-500 rounded-lg hover:bg-red-500/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                禁用通知
              </button>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={onClose}
              className="px-6 py-2 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-colors"
            >
              取消
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving || isLoading}
              className="px-6 py-2 bg-gradient-to-r from-[#F0B90B] to-[#E1A706] text-black rounded-lg hover:from-[#E1A706] hover:to-[#D4951E] transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
            >
              {isSaving ? '保存中...' : '保存配置'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
