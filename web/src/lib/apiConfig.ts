/**
 * API 配置模块
 * 统一管理所有 API 相关配置
 * 确保前端数据都从后端 API 获取，不直接访问数据库
 */

// 默认后端 API 地址
const DEFAULT_API_URL = 'https://nofx-gyc567.replit.app';

/**
 * 获取后端 API 基础 URL
 * 开发环境使用相对路径，生产环境使用绝对路径
 */
export function getApiBaseUrl(): string {
  // 生产环境直接调用后端API
  // 注意: 需要后端配置CORS允许Vercel域名
  const apiUrl = import.meta.env.VITE_API_URL || DEFAULT_API_URL;
  return `${apiUrl}/api`;
}

/**
 * 获取后端 API 完整 URL
 * @param endpoint API 端点（如 '/supported-exchanges'）
 * @returns 完整的 API URL
 */
export function getApiUrl(endpoint: string): string {
  // 移除开头多余的斜杠
  const cleanEndpoint = endpoint.startsWith('/') ? endpoint.slice(1) : endpoint;
  return `${getApiBaseUrl()}/${cleanEndpoint}`;
}

/**
 * 获取后端基础域名
 * @returns 后端域名（如 'https://nofx-gyc567.replit.app'）
 */
export function getBackendUrl(): string {
  if (import.meta.env.DEV) {
    return ''; // 开发环境不需要域名
  }

  const apiUrl = import.meta.env.VITE_API_URL || DEFAULT_API_URL;
  return apiUrl;
}

/**
 * 检查是否为开发环境
 */
export function isDevelopment(): boolean {
  return import.meta.env.DEV;
}

/**
 * 检查是否使用环境变量中的 API URL
 */
export function isUsingEnvironmentApiUrl(): boolean {
  return !!import.meta.env.VITE_API_URL;
}
