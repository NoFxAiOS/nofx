import React from 'react'

interface Props {
  children: React.ReactNode
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends React.Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div
          style={{
            minHeight: '100vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#0B0E11',
            color: '#EAECEF',
            fontFamily: 'system-ui, sans-serif',
          }}
        >
          <div
            style={{ textAlign: 'center', maxWidth: 400, padding: '0 24px' }}
          >
            <h2 style={{ fontSize: 20, marginBottom: 12 }}>页面加载失败</h2>
            <p style={{ color: '#848E9C', marginBottom: 24, fontSize: 14 }}>
              应用发生了意外错误，请刷新页面重试。如果问题持续，请尝试清除浏览器缓存。
            </p>
            <button
              onClick={() => {
                this.setState({ hasError: false })
                window.location.reload()
              }}
              style={{
                padding: '10px 24px',
                borderRadius: 8,
                border: '1px solid rgba(240, 185, 11, 0.3)',
                background: 'rgba(240, 185, 11, 0.1)',
                color: '#F0B90B',
                cursor: 'pointer',
                fontSize: 14,
                fontWeight: 600,
              }}
            >
              刷新页面
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
