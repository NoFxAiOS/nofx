import { useEffect, useRef, useState } from 'react'

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

  useEffect(() => {
    function typeNext() {
      const currentLine = lines[lineIndexRef.current] ?? ''
      if (charIndexRef.current < currentLine.length) {
        setTypedLines((prev) => {
          const next = [...prev]
          next[next.length - 1] = (next[next.length - 1] || '') + currentLine[charIndexRef.current]
          return next
        })
        charIndexRef.current += 1
        timerRef.current = window.setTimeout(typeNext, typingSpeed)
      } else {
        // 行结束
        if (lineIndexRef.current < lines.length - 1) {
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

    typeNext()

    // 光标闪烁
    blinkRef.current = window.setInterval(() => {
      setShowCursor((v) => !v)
    }, 500)

    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current)
      if (blinkRef.current) window.clearInterval(blinkRef.current)
    }
  }, [lines, typingSpeed, lineDelay])

  return (
    <pre className={className} style={style}>
      {typedLines.join('\n')}
      <span style={{ opacity: showCursor ? 1 : 0 }}> ▍</span>
    </pre>
  )
}
