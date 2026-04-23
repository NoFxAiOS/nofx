import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

interface PerformanceContextType {
  lowPerformanceMode: boolean
  setLowPerformanceMode: (enabled: boolean) => void
  toggleLowPerformanceMode: () => void
}

const PerformanceContext = createContext<PerformanceContextType | undefined>(undefined)
const STORAGE_KEY = 'nofx.lowPerformanceMode'

export function PerformanceProvider({ children }: { children: ReactNode }) {
  const [lowPerformanceMode, setLowPerformanceModeState] = useState<boolean>(() => {
    const saved = localStorage.getItem(STORAGE_KEY)
    return saved === 'true'
  })

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(lowPerformanceMode))
    document.documentElement.dataset.lowPerf = lowPerformanceMode ? 'true' : 'false'
  }, [lowPerformanceMode])

  const value = useMemo<PerformanceContextType>(() => ({
    lowPerformanceMode,
    setLowPerformanceMode: setLowPerformanceModeState,
    toggleLowPerformanceMode: () => setLowPerformanceModeState((prev) => !prev),
  }), [lowPerformanceMode])

  return (
    <PerformanceContext.Provider value={value}>
      {children}
    </PerformanceContext.Provider>
  )
}

export function usePerformanceMode() {
  const context = useContext(PerformanceContext)
  if (!context) {
    throw new Error('usePerformanceMode must be used within PerformanceProvider')
  }
  return context
}
