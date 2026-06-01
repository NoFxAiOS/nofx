import { traderApi } from './traders'
import { strategyApi } from './strategies'
import { configApi } from './config'
import { dataApi } from './data'
import { telegramApi } from './telegram'
import { walletApi } from './wallet'

export const api = {
  ...traderApi,
  ...strategyApi,
  ...configApi,
  ...dataApi,
  ...telegramApi,
  ...walletApi,
}
