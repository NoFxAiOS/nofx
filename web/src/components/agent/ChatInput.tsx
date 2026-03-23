import { useRef, useState, useCallback, useEffect, useImperativeHandle, forwardRef } from 'react'
import { ArrowUp } from 'lucide-react'

export interface ChatInputHandle {
  focus: () => void
  clear: () => void
  getValue: () => string
}

interface ChatInputProps {
  language: string
  loading: boolean
  onSend: (text: string) => void
}

export const ChatInput = forwardRef<ChatInputHandle, ChatInputProps>(
  function ChatInput({ language, loading, onSend }, ref) {
    const [input, setInput] = useState('')
    const [composing, setComposing] = useState(false)
    const inputRef = useRef<HTMLTextAreaElement>(null)

    useImperativeHandle(ref, () => ({
      focus: () => inputRef.current?.focus(),
      clear: () => {
        setInput('')
        if (inputRef.current) inputRef.current.style.height = 'auto'
      },
      getValue: () => input,
    }))

    const handleInputChange = useCallback(
      (e: React.ChangeEvent<HTMLTextAreaElement>) => {
        setInput(e.target.value)
        const el = e.target
        el.style.height = 'auto'
        el.style.height = Math.min(el.scrollHeight, 150) + 'px'
      },
      []
    )

    const handleSend = () => {
      const msg = input.trim()
      if (!msg || loading) return
      setInput('')
      if (inputRef.current) inputRef.current.style.height = 'auto'
      onSend(msg)
      inputRef.current?.focus()
    }

    // Keyboard shortcut: Cmd+K to focus
    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
          e.preventDefault()
          inputRef.current?.focus()
        }
      }
      window.addEventListener('keydown', handleKeyDown)
      return () => window.removeEventListener('keydown', handleKeyDown)
    }, [])

    return (
      <div
        style={{
          padding: '12px 16px 20px',
          borderTop: '1px solid rgba(255,255,255,0.04)',
          background: 'linear-gradient(to top, #09090b 80%, transparent)',
        }}
      >
        <div
          className="chat-input-wrapper"
          style={{
            maxWidth: 720,
            margin: '0 auto',
            display: 'flex',
            gap: 8,
            background: 'rgba(255,255,255,0.03)',
            border: '1px solid rgba(255,255,255,0.07)',
            borderRadius: 18,
            padding: '4px 4px 4px 16px',
            alignItems: 'flex-end',
            transition: 'all 0.2s ease',
          }}
        >
          <textarea
            ref={inputRef}
            value={input}
            onChange={handleInputChange}
            onCompositionStart={() => setComposing(true)}
            onCompositionEnd={() => setComposing(false)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && !composing) {
                e.preventDefault()
                handleSend()
              }
            }}
            placeholder={
              language === 'zh'
                ? '跟 NOFXi 聊点什么...  ⌘K'
                : 'Ask NOFXi anything...  ⌘K'
            }
            rows={1}
            style={{
              flex: 1,
              background: 'none',
              border: 'none',
              color: '#eaeaf0',
              fontSize: 13.5,
              outline: 'none',
              padding: '10px 0',
              fontFamily: 'inherit',
              resize: 'none',
              lineHeight: 1.5,
              maxHeight: 150,
            }}
          />
          <button
            onClick={handleSend}
            disabled={loading || !input.trim()}
            style={{
              width: 36,
              height: 36,
              borderRadius: 12,
              border: 'none',
              background:
                loading || !input.trim()
                  ? 'rgba(255,255,255,0.04)'
                  : 'linear-gradient(135deg, #F0B90B, #d4a30a)',
              color: loading || !input.trim() ? '#3c3c52' : '#000',
              cursor: loading || !input.trim() ? 'not-allowed' : 'pointer',
              display: 'grid',
              placeItems: 'center',
              flexShrink: 0,
              transition: 'all 0.2s ease',
            }}
          >
            <ArrowUp size={16} strokeWidth={2.5} />
          </button>
        </div>
        <div
          style={{
            maxWidth: 720,
            margin: '6px auto 0',
            textAlign: 'center',
            fontSize: 10,
            color: '#1e1e32',
          }}
        >
          NOFXi may make mistakes. Always verify trading decisions.
        </div>
      </div>
    )
  }
)
