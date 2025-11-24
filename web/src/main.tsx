import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import { Toaster } from 'sonner'
import './index.ligth.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Toaster
      theme="light"
      richColors
      closeButton
      position="top-center"
      duration={2200}
      toastOptions={{
            className: 'ares-toast',
        style: {
          background: '#FAFAFA',
          border: '1px solid var(--panel-border)',
          color: 'var(--text-primary)',
        },
      }}
    />
    <App />
  </React.StrictMode>
)
