import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import { Toaster } from 'sonner'
import './index.css'
import { BrowserRouter } from 'react-router-dom'
import { ErrorBoundary } from './components/ErrorBoundary'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <BrowserRouter>
        <Toaster
          theme="dark"
          richColors
          closeButton
          position="top-center"
          duration={2200}
          toastOptions={{
            className: 'nofx-toast',
            style: {
              background: '#0b0e11',
              border: '1px solid var(--panel-border)',
              color: 'var(--text-primary)',
            },
          }}
        />
        <App />
      </BrowserRouter>
    </ErrorBoundary>
  </React.StrictMode>
)
