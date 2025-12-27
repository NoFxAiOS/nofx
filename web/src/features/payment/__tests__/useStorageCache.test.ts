/**
 * useStorageCache 单元测试
 * 测试：缓存读写、TTL 过期、错误处理
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createStorageCache } from '../hooks/useStorageCache'

describe('useStorageCache [M1 缓存层分离验证]', () => {
  const CACHE_KEY = 'test-cache'
  const SHORT_TTL = 100 // 100ms，便于测试过期
  const LONG_TTL = 60000 // 60秒

  beforeEach(() => {
    // 清空所有缓存
    localStorage.clear()
    vi.clearAllTimers()
  })

  afterEach(() => {
    localStorage.clear()
  })

  describe('基础读写操作', () => {
    it('应该能够存储和检索数据', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)
      const testData = 'test-value'

      cache.set(testData)
      const retrieved = cache.get()

      expect(retrieved).toBe(testData)
    })

    it('应该返回复杂对象', () => {
      const cache = createStorageCache<{ id: string; name: string }>(CACHE_KEY, LONG_TTL)
      const testData = { id: '123', name: 'Test' }

      cache.set(testData)
      const retrieved = cache.get()

      expect(retrieved).toEqual(testData)
    })

    it('应该返回数组', () => {
      const cache = createStorageCache<string[]>(CACHE_KEY, LONG_TTL)
      const testData = ['item1', 'item2', 'item3']

      cache.set(testData)
      const retrieved = cache.get()

      expect(Array.isArray(retrieved)).toBe(true)
      expect(retrieved).toEqual(testData)
    })

    it('获取不存在的缓存应该返回 null', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)

      const result = cache.get()

      expect(result).toBeNull()
    })
  })

  describe('TTL 过期处理', () => {
    it('缓存过期时应该返回 null', async () => {
      vi.useFakeTimers()

      const cache = createStorageCache<string>(CACHE_KEY, SHORT_TTL)
      cache.set('test-value')

      // 初次获取应该成功
      expect(cache.get()).toBe('test-value')

      // 推进时间超过 TTL
      vi.advanceTimersByTime(SHORT_TTL + 1)

      // 现在应该返回 null
      expect(cache.get()).toBeNull()

      vi.useRealTimers()
    })

    it('清除缓存应该返回 null', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)
      cache.set('test-value')

      expect(cache.get()).toBe('test-value')

      cache.clear()

      expect(cache.get()).toBeNull()
    })

    it('多个缓存应该独立管理', () => {
      const cache1 = createStorageCache<string>('key1', LONG_TTL)
      const cache2 = createStorageCache<string>('key2', LONG_TTL)

      cache1.set('value1')
      cache2.set('value2')

      expect(cache1.get()).toBe('value1')
      expect(cache2.get()).toBe('value2')

      cache1.clear()

      expect(cache1.get()).toBeNull()
      expect(cache2.get()).toBe('value2')
    })
  })

  describe('错误处理', () => {
    it('应该优雅处理无效的 JSON', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)

      // 直接在 localStorage 存入无效 JSON
      localStorage.setItem(CACHE_KEY, 'invalid-json')

      // 应该返回 null 而不是抛出错误
      const result = cache.get()

      expect(result).toBeNull()
    })

    it('应该处理 localStorage 满的情况', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)

      // Mock localStorage.setItem 抛出错误
      vi.spyOn(Storage.prototype, 'setItem').mockImplementationOnce(() => {
        throw new Error('QuotaExceededError')
      })

      // 应该不抛出异常，继续执行
      expect(() => {
        cache.set('test-value')
      }).not.toThrow()

      // 数据不会被缓存，但程序继续运行
      expect(cache.get()).toBeNull()
    })

    it('应该处理数据包含特殊字符', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)
      const specialData = '{"nested":"value"}\\n\\t\\u0000'

      cache.set(specialData)
      const retrieved = cache.get()

      expect(retrieved).toBe(specialData)
    })
  })

  describe('性能和内存', () => {
    it('应该能处理大数据', () => {
      const cache = createStorageCache<string[]>(CACHE_KEY, LONG_TTL)
      const largeArray = Array.from({ length: 1000 }, (_, i) => `item-${i}`)

      cache.set(largeArray)
      const retrieved = cache.get()

      expect(retrieved?.length).toBe(1000)
      expect(retrieved?.[500]).toBe('item-500')
    })

    it('应该快速检索缓存', () => {
      const cache = createStorageCache<{ id: number; data: string }>(CACHE_KEY, LONG_TTL)
      const testData = { id: 1, data: 'test' }

      cache.set(testData)

      const start = performance.now()
      for (let i = 0; i < 1000; i++) {
        cache.get()
      }
      const duration = performance.now() - start

      // 应该在 150ms 内完成 1000 次读取（合理的性能基准）
      expect(duration).toBeLessThan(150)
    })
  })

  describe('类型安全', () => {
    it('应该维护类型信息', () => {
      const cache = createStorageCache<{ name: string; age: number }>(CACHE_KEY, LONG_TTL)
      const data = { name: 'John', age: 30 }

      cache.set(data)
      const retrieved = cache.get()

      // TypeScript 应该知道 retrieved 的类型
      expect(typeof retrieved?.name).toBe('string')
      expect(typeof retrieved?.age).toBe('number')
    })

    it('应该处理 null 类型安全', () => {
      const cache = createStorageCache<string>(CACHE_KEY, LONG_TTL)

      const result = cache.get()

      // result 应该是 T | null，不是 T
      expect(result === null).toBe(true)
    })
  })

  describe('实际使用场景', () => {
    it('应该支持 API 响应缓存场景', () => {
      interface ApiResponse {
        packages: { id: string; name: string }[]
        timestamp: number
      }

      const cache = createStorageCache<ApiResponse>('pricing-data', 5 * 60 * 1000)

      const response: ApiResponse = {
        packages: [
          { id: 'starter', name: 'Starter Package' },
          { id: 'pro', name: 'Pro Package' }
        ],
        timestamp: Date.now()
      }

      cache.set(response)
      const cached = cache.get()

      expect(cached?.packages).toHaveLength(2)
      expect(cached?.packages[0].id).toBe('starter')
    })

    it('应该支持用户偏好缓存', () => {
      interface UserPreferences {
        theme: 'light' | 'dark'
        language: string
      }

      const cache = createStorageCache<UserPreferences>('user-prefs', 24 * 60 * 60 * 1000)

      const prefs: UserPreferences = {
        theme: 'dark',
        language: 'zh-CN'
      }

      cache.set(prefs)
      const cached = cache.get()

      expect(cached?.theme).toBe('dark')
      expect(cached?.language).toBe('zh-CN')
    })
  })
})
