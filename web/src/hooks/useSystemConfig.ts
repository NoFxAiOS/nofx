import { useEffect, useState, useCallback } from 'react'
import { getSystemConfig, invalidateSystemConfig, type SystemConfig } from '../lib/config'

export function useSystemConfig() {
  const [config, setConfig] = useState<SystemConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [fetchKey, setFetchKey] = useState(0)

  useEffect(() => {
    let mounted = true
    setLoading(true)
    getSystemConfig()
      .then((data) => {
        if (!mounted) return
        setConfig(data)
        setLoading(false)
      })
      .catch((err: Error) => {
        if (!mounted) return
        console.error('Failed to fetch system config:', err)
        setError(err.message)
        setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [fetchKey])

  // Listen for invalidation events and re-fetch automatically
  useEffect(() => {
    const handler = () => setFetchKey((k) => k + 1)
    window.addEventListener('system-config-invalidated', handler)
    return () => window.removeEventListener('system-config-invalidated', handler)
  }, [])

  const refresh = useCallback(() => {
    invalidateSystemConfig()
  }, [])

  return { config, loading, error, refresh }
}
