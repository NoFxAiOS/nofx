/**
 * MessageRenderer — markdown-to-JSX renderer for agent chat messages.
 * Supports: headers, bold, italic, inline code, code blocks, lists, links, HR.
 */

// Inline formatting: bold, italic, code, links
export function renderInline(text: string): (string | JSX.Element)[] {
  const parts = text.split(/(```[\s\S]*?```|`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|\[([^\]]+)\]\(([^)]+)\))/g)
  const result: (string | JSX.Element)[] = []

  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]
    if (!part) continue

    if (part.startsWith('`') && part.endsWith('`') && !part.startsWith('```')) {
      result.push(
        <code
          key={i}
          style={{
            background: 'rgba(240,185,11,0.08)',
            padding: '2px 6px',
            borderRadius: 5,
            fontSize: '0.88em',
            fontFamily: '"IBM Plex Mono", monospace',
            color: '#F0B90B',
            border: '1px solid rgba(240,185,11,0.12)',
          }}
        >
          {part.slice(1, -1)}
        </code>
      )
    } else if (part.startsWith('**') && part.endsWith('**')) {
      result.push(
        <strong key={i} style={{ fontWeight: 600, color: '#f0f0f8' }}>
          {part.slice(2, -2)}
        </strong>
      )
    } else if (part.startsWith('*') && part.endsWith('*') && !part.startsWith('**')) {
      result.push(
        <em key={i} style={{ fontStyle: 'italic', color: '#d0d0e0' }}>
          {part.slice(1, -1)}
        </em>
      )
    } else if (part.match(/^\[([^\]]+)\]\(([^)]+)\)$/)) {
      const match = part.match(/^\[([^\]]+)\]\(([^)]+)\)$/)
      if (match) {
        const href = match[2]
        // Only allow http/https links to prevent javascript: XSS
        const safeHref = /^https?:\/\//i.test(href) ? href : '#'
        result.push(
          <a
            key={i}
            href={safeHref}
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: '#F0B90B', textDecoration: 'underline', textUnderlineOffset: 2 }}
          >
            {match[1]}
          </a>
        )
      }
    } else {
      result.push(part)
    }
  }

  return result
}

// Enhanced markdown renderer: headers, bold, italic, code, lists, links
export function renderMessageContent(text: string) {
  const lines = text.split('\n')
  const elements: JSX.Element[] = []
  let inCodeBlock = false
  let codeContent = ''

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Code block toggle
    if (line.startsWith('```')) {
      if (inCodeBlock) {
        elements.push(
          <pre
            key={`code-${i}`}
            style={{
              background: '#0a0a12',
              border: '1px solid rgba(255,255,255,0.06)',
              borderRadius: 10,
              padding: '12px 14px',
              fontSize: 12,
              overflowX: 'auto',
              margin: '8px 0',
              fontFamily: '"IBM Plex Mono", monospace',
              color: '#c0c0d0',
              lineHeight: 1.6,
            }}
          >
            {codeContent.trim()}
          </pre>
        )
        codeContent = ''
        inCodeBlock = false
      } else {
        inCodeBlock = true
      }
      continue
    }

    if (inCodeBlock) {
      codeContent += (codeContent ? '\n' : '') + line
      continue
    }

    // Headers
    if (line.startsWith('### ')) {
      elements.push(
        <div key={i} style={{ fontSize: 14, fontWeight: 700, color: '#f0f0f8', margin: '12px 0 6px', letterSpacing: '-0.01em' }}>
          {renderInline(line.slice(4))}
        </div>
      )
      continue
    }
    if (line.startsWith('## ')) {
      elements.push(
        <div key={i} style={{ fontSize: 15, fontWeight: 700, color: '#f0f0f8', margin: '14px 0 6px', letterSpacing: '-0.01em' }}>
          {renderInline(line.slice(3))}
        </div>
      )
      continue
    }
    if (line.startsWith('# ')) {
      elements.push(
        <div key={i} style={{ fontSize: 16, fontWeight: 700, color: '#f0f0f8', margin: '16px 0 8px', letterSpacing: '-0.02em' }}>
          {renderInline(line.slice(2))}
        </div>
      )
      continue
    }

    // Bullet lists
    if (line.match(/^[-•*]\s/)) {
      elements.push(
        <div key={i} style={{ display: 'flex', gap: 8, padding: '2px 0', lineHeight: 1.65 }}>
          <span style={{ color: '#F0B90B', flexShrink: 0, fontSize: 8, marginTop: 7 }}>●</span>
          <span>{renderInline(line.replace(/^[-•*]\s/, ''))}</span>
        </div>
      )
      continue
    }

    // Numbered lists
    if (line.match(/^\d+\.\s/)) {
      const num = line.match(/^(\d+)\./)?.[1]
      elements.push(
        <div key={i} style={{ display: 'flex', gap: 8, padding: '2px 0', lineHeight: 1.65 }}>
          <span style={{ color: '#8a8aa0', flexShrink: 0, fontSize: 12, fontWeight: 600, minWidth: 16, fontFamily: '"IBM Plex Mono", monospace' }}>{num}.</span>
          <span>{renderInline(line.replace(/^\d+\.\s/, ''))}</span>
        </div>
      )
      continue
    }

    // Horizontal rule
    if (line.match(/^---+$/)) {
      elements.push(
        <hr key={i} style={{ border: 'none', borderTop: '1px solid rgba(255,255,255,0.06)', margin: '12px 0' }} />
      )
      continue
    }

    // Empty line → small gap
    if (line.trim() === '') {
      elements.push(<div key={i} style={{ height: 6 }} />)
      continue
    }

    // Regular paragraph
    elements.push(
      <div key={i} style={{ lineHeight: 1.7, padding: '1px 0' }}>
        {renderInline(line)}
      </div>
    )
  }

  return elements
}
