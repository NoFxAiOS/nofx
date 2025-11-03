import { z } from 'zod';
import { translations } from '../i18n/translations';
import type { Language } from '../i18n/translations';

/**
 * Validation schemas using Zod for form validation
 * Based on README.md requirements
 */

// Helper function to get translated message
const t = (key: keyof typeof translations.en, lang: Language = 'zh') => {
  return translations[lang][key] || translations.en[key];
};

// AI Model Configuration Schema factory
export const createAIModelSchema = (language: Language = 'zh') => z.object({
  modelId: z.string().min(1, t('fieldRequired', language)),
  apiKey: z
    .string()
    .min(1, t('apiKeyRequired', language))
    .startsWith('sk-', t('apiKeyMustStartWithSk', language))
    .min(10, t('apiKeyInvalidFormat', language)),
  baseUrl: z
    .string()
    .url(t('baseUrlMustBeValid', language))
    .startsWith('http', t('baseUrlMustStartWithHttp', language))
    .optional()
    .or(z.literal('')), // Allow empty string
  modelName: z
    .string()
    .regex(
      /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/,
      t('modelNameInvalidFormat', language)
    )
    .optional()
    .or(z.literal('')), // Allow empty string
});

// Default schema for backward compatibility
export const aiModelSchema = createAIModelSchema('zh');

export type AIModelFormData = z.infer<typeof aiModelSchema>;

// Binance Exchange Schema
export const binanceExchangeSchema = z.object({
  exchangeId: z.literal('binance'),
  apiKey: z.string().min(1, 'API Key 是必填项'),
  secretKey: z.string().min(1, 'Secret Key 是必填项'),
  testnet: z.boolean().optional(),
});

// OKX Exchange Schema
export const okxExchangeSchema = z.object({
  exchangeId: z.literal('okx'),
  apiKey: z.string().min(1, 'API Key 是必填项'),
  secretKey: z.string().min(1, 'Secret Key 是必填项'),
  passphrase: z.string().min(1, 'Passphrase 是必填项'),
  testnet: z.boolean().optional(),
});

// Hyperliquid Exchange Schema
export const hyperliquidExchangeSchema = z.object({
  exchangeId: z.literal('hyperliquid'),
  privateKey: z
    .string()
    .min(1, 'Private Key 是必填项')
    .regex(/^[a-fA-F0-9]{64}$/, 'Private Key 必须是 64 位十六进制字符（不含 0x 前缀）')
    .refine((val) => !val.startsWith('0x'), {
      message: 'Private Key 不应包含 0x 前缀',
    }),
  walletAddress: z
    .string()
    .min(1, 'Wallet Address 是必填项')
    .regex(/^0x[a-fA-F0-9]{40}$/, 'Wallet Address 必须是有效的以太坊地址（0x + 40位十六进制）'),
  testnet: z.boolean().optional(),
});

// Aster DEX Exchange Schema
export const asterExchangeSchema = z.object({
  exchangeId: z.literal('aster'),
  user: z
    .string()
    .min(1, '用户名是必填项')
    .regex(/^0x[a-fA-F0-9]{40}$/, '用户名必须是有效的以太坊地址（0x + 40位十六进制）'),
  signer: z
    .string()
    .min(1, '签名者是必填项')
    .regex(/^0x[a-fA-F0-9]{40}$/, '签名者必须是有效的以太坊地址（0x + 40位十六进制）'),
  privateKey: z
    .string()
    .min(1, 'Private Key 是必填项')
    .regex(/^[a-fA-F0-9]{64}$/, 'Private Key 必须是 64 位十六进制字符（不含 0x 前缀）')
    .refine((val) => !val.startsWith('0x'), {
      message: 'Private Key 不应包含 0x 前缀',
    }),
  testnet: z.boolean().optional(),
});

// Generic CEX Exchange Schema (for other exchanges)
export const genericCexExchangeSchema = z.object({
  exchangeId: z.string().min(1),
  apiKey: z.string().min(1, 'API Key 是必填项'),
  secretKey: z.string().min(1, 'Secret Key 是必填项'),
  testnet: z.boolean().optional(),
});

// Union type for all exchange schemas
export const exchangeSchema = z.discriminatedUnion('exchangeId', [
  binanceExchangeSchema,
  okxExchangeSchema,
  hyperliquidExchangeSchema,
  asterExchangeSchema,
]);

export type ExchangeFormData =
  | z.infer<typeof binanceExchangeSchema>
  | z.infer<typeof okxExchangeSchema>
  | z.infer<typeof hyperliquidExchangeSchema>
  | z.infer<typeof asterExchangeSchema>;

// Helper function to get the appropriate schema for an exchange
export function getExchangeSchema(exchangeId: string) {
  switch (exchangeId) {
    case 'binance':
      return binanceExchangeSchema;
    case 'okx':
      return okxExchangeSchema;
    case 'hyperliquid':
      return hyperliquidExchangeSchema;
    case 'aster':
      return asterExchangeSchema;
    default:
      return genericCexExchangeSchema;
  }
}
