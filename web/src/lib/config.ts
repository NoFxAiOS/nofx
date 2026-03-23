import { httpClient, type ApiResponse } from './httpClient'

export interface SystemConfig {
  initialized: boolean
  beta_mode?: boolean
}

let configPromise: Promise<SystemConfig> | null = null
let cachedConfig: SystemConfig | null = null

export function getSystemConfig(): Promise<SystemConfig> {
  if (cachedConfig) {
    return Promise.resolve(cachedConfig)
  }
  if (configPromise) {
    return configPromise
  }

  configPromise = httpClient
    .get<SystemConfig>('/api/config')
    .then((result: ApiResponse<SystemConfig>) => {
      if (!result.success || !result.data) {
        throw new Error(result.message || 'Failed to fetch system config')
      }
      cachedConfig = result.data
      return result.data
    })

  return configPromise
}

/** Call after first-time setup completes so next check reflects initialized=true */
export function invalidateSystemConfig() {
  cachedConfig = null
  configPromise = null
}
