import { useEffect, useMemo, useRef, useState } from 'react'

interface TypewriterProps {
  lines: string[]
  typingSpeed?: number // 毫秒/字符
  lineDelay?: number // 每行结束的额外等待
  className?: string
  style?: React.CSSProperties
}

export default function Typewriter({
  lines,
  typingSpeed = 50,
  lineDelay = 600,
  className,
  style,
}: TypewriterProps) {
  const [typedLines, setTypedLines] = useState<string[]>([''])
  const [showCursor, setShowCursor] = useState(true)
  const lineIndexRef = useRef(0)
  const charIndexRef = useRef(0)
  const timerRef = useRef<number | null>(null)
  const blinkRef = useRef<number | null>(null)
  const sanitizedLines = useMemo(
    () => lines.map((l) => String(l ?? '')),
    [lines]
  )

  useEffect(() => {
    // 重置状态
    lineIndexRef.current = 0
    charIndexRef.current = 0
    setTypedLines([''])

    function typeNext() {
      const currentLine = sanitizedLines[lineIndexRef.current] ?? ''
      if (charIndexRef.current < currentLine.length) {
        const ch = currentLine.charAt(charIndexRef.current)
        setTypedLines((prev) => {
          const next = [...prev]
          const lastIndex = next.length - 1
          next[lastIndex] = (next[lastIndex] ?? '') + ch
          return next
        })
        charIndexRef.current += 1
        timerRef.current = window.setTimeout(typeNext, typingSpeed)
      } else {
        // 行结束
        if (lineIndexRef.current < sanitizedLines.length - 1) {
          lineIndexRef.current += 1
          charIndexRef.current = 0
          setTypedLines((prev) => [...prev, ''])
          timerRef.current = window.setTimeout(typeNext, lineDelay)
        } else {
          // 最后一行输入完毕
          timerRef.current = null
        }
      }
    }

    // 延迟一帧开始打字,确保状态已重置
    timerRef.current = window.setTimeout(typeNext, 0)

    // 光标闪烁
    blinkRef.current = window.setInterval(() => {
      setShowCursor((v) => !v)
    }, 500)

    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current)
      if (blinkRef.current) window.clearInterval(blinkRef.current)
    }
  }, [sanitizedLines, typingSpeed, lineDelay])

  const displayText = useMemo(
    () => typedLines.join('\n').replace(/undefined/g, ''),
    [typedLines]
  )

  return (
    <pre className={className} style={{ whiteSpace: 'pre-wrap', ...style }}>
      {displayText}
      <span style={{ opacity: showCursor ? 1 : 0 }}> ▍</span>
    </pre>
  )
}
