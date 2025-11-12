import '@testing-library/jest-dom'
import { beforeAll, afterEach } from 'vitest'

// Mock localStorage
const localStorageMock = {
  getItem: (key: string) => {
    return localStorageMock._store[key] || null
  },
  setItem: (key: string, value: string) => {
    localStorageMock._store[key] = value
  },
  removeItem: (key: string) => {
    delete localStorageMock._store[key]
  },
  clear: () => {
    localStorageMock._store = {}
  },
  _store: {} as Record<string, string>,
}

// Setup before all tests
beforeAll(() => {
  Object.defineProperty(window, 'localStorage', {
    value: localStorageMock,
    writable: true,
  })
})

// Clean up after each test
afterEach(() => {
  localStorageMock.clear()
})
