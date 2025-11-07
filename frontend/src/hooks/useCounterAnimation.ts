import { useEffect, useState } from 'react'

interface UseCounterAnimationOptions {
  start?: number
  end: number
  duration?: number
  decimals?: number
}

export function useCounterAnimation({
  start = 0,
  end,
  duration = 2000,
  decimals = 0,
}: UseCounterAnimationOptions): number {
  const [count, setCount] = useState(start)

  useEffect(() => {
    if (end === 0) return

    let startTime: number | null = null
    let animationFrame: number

    const animate = (currentTime: number) => {
      if (startTime === null) startTime = currentTime
      const progress = Math.min((currentTime - startTime) / duration, 1)

      // 使用 easeOutExpo 缓动函数，让数字快速启动后缓慢停止
      const easeOutExpo = progress === 1 ? 1 : 1 - Math.pow(2, -10 * progress)

      const currentCount = start + (end - start) * easeOutExpo
      setCount(currentCount)

      if (progress < 1) {
        animationFrame = requestAnimationFrame(animate)
      } else {
        setCount(end)
      }
    }

    animationFrame = requestAnimationFrame(animate)

    return () => {
      if (animationFrame) {
        cancelAnimationFrame(animationFrame)
      }
    }
  }, [start, end, duration])

  return decimals > 0 ? parseFloat(count.toFixed(decimals)) : Math.floor(count)
}
