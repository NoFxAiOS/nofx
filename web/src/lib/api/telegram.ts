import type { TelegramConfig } from '../../types'
import { API_BASE, httpClient } from './helpers'

export const telegramApi = {
  async getTelegramConfig(): Promise<TelegramConfig> {
    const result = await httpClient.get<TelegramConfig>(`${API_BASE}/telegram`)
    if (!result.success) throw new Error('获取Telegram配置失败')
    return result.data!
  },

  async updateTelegramConfig(token: string, modelId?: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/telegram`, { bot_token: token, model_id: modelId ?? '' })
    if (!result.success) throw new Error('保存Telegram配置失败')
  },

  async unbindTelegram(): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/telegram/binding`)
    if (!result.success) throw new Error('解绑Telegram失败')
  },

  async updateTelegramModel(modelId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/telegram/model`, { model_id: modelId })
    if (!result.success) throw new Error('更新Telegram模型失败')
  },
}
