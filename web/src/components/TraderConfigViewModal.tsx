import type { TraderConfigData } from '../types'
import { PunkAvatar, getTraderAvatar } from './PunkAvatar'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface TraderConfigViewModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
}

export function TraderConfigViewModal({
  isOpen,
  onClose,
  traderData,
}: TraderConfigViewModalProps) {
  if (!isOpen || !traderData) return null

  const InfoRow = ({
    label,
    value,
  }: {
    label: string
    value: string | number | boolean
  }) => (
    <div className="flex justify-between items-start py-2 border-b border-[#2B3139] last:border-b-0">
      <span className="text-sm text-[#848E9C] font-medium">{label}</span>
      <span className="text-sm text-[#EAECEF] font-mono text-right">
        {typeof value === 'boolean' ? (value ? '是' : '否') : value}
      </span>
    </div>
  )

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 backdrop-blur-sm">
      <div
        className="bg-[#1E2329] border border-[#2B3139] rounded-xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35]">
          <div className="flex items-center gap-3">
            <PunkAvatar
              seed={getTraderAvatar(traderData.trader_id || '', traderData.trader_name)}
              size={48}
              className="rounded-lg"
            />
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">交易员配置</h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {traderData.trader_name} 的配置信息
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Running Status */}
            <div
              className="px-3 py-1 rounded-full text-xs font-bold flex items-center gap-1"
              style={
                traderData.is_running
                  ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                  : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
              }
            >
              <span>{traderData.is_running ? '●' : '○'}</span>
              {traderData.is_running ? '运行中' : '已停止'}
            </div>
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center"
            >
              ✕
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {/* Basic Info */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
              🤖 基础信息
            </h3>
            <div className="space-y-3">
              <InfoRow
                label="交易员名称"
                value={traderData.trader_name}
              />
              <InfoRow
                label="AI模型"
                value={getShortName(traderData.ai_model).toUpperCase()}
              />
              <InfoRow
                label="交易所"
                value={getShortName(traderData.exchange_id).toUpperCase()}
              />
              <InfoRow
                label="初始余额"
                value={`$${traderData.initial_balance.toLocaleString()}`}
              />
              <InfoRow
                label="保证金模式"
                value={traderData.is_cross_margin ? '全仓' : '逐仓'}
              />
              <InfoRow
                label="扫描间隔"
                value={`${traderData.scan_interval_minutes || 3} 分钟`}
              />
            </div>
          </div>

          {/* Strategy Info - only show if strategy is bound */}
          {traderData.strategy_id && (
            <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
                📋 使用策略
              </h3>
              <div className="space-y-3">
                <InfoRow
                  label="策略名称"
                  value={traderData.strategy_name || traderData.strategy_id}
                />
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end p-6 border-t border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35]">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-all duration-200 border border-[#404750]"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  )
}
