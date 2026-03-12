import { traderApi } from './traders'
import { backtestApi } from './backtest'
import { strategyApi } from './strategies'
import { configApi } from './config'
import { dataApi } from './data'
import { telegramApi } from './telegram'

export const api = {
  ...traderApi,
  ...backtestApi,
  ...strategyApi,
  ...configApi,
  ...dataApi,
  ...telegramApi,
}
