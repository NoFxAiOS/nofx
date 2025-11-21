// pages/test-connection.js
import { useEffect, useState } from 'react'

export default function TestConnection() {
  const [status, setStatus] = useState('准备测试...')
  const [details, setDetails] = useState('')

  useEffect(() => {
    async function testConnection() {
      try {
        setStatus('🔄 正在测试 Supabase 连接...')
        
        // 这里需要您的 Supabase 客户端代码
        // 暂时先模拟测试
        setTimeout(() => {
          if (process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY) {
            setStatus('✅ 环境变量已设置，可以尝试连接')
            setDetails(`URL: ${process.env.NEXT_PUBLIC_SUPABASE_URL}\n密钥长度: ${process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY.length}`)
          } else {
            setStatus('❌ 环境变量未正确设置')
          }
        }, 1000)
        
      } catch (err) {
        setStatus(`💥 测试失败: ${err.message}`)
      }
    }

    testConnection()
  }, [])

  return (
    <div style={{ padding: '20px' }}>
      <h1>Supabase 连接测试</h1>
      <div style={{ 
        background: '#f8f9fa', 
        padding: '20px', 
        borderRadius: '8px',
        marginBottom: '20px'
      }}>
        <h2>测试状态</h2>
        <p>{status}</p>
        {details && (
          <pre style={{ 
            background: '#fff', 
            padding: '10px', 
            borderRadius: '4px',
            marginTop: '10px',
            whiteSpace: 'pre-wrap'
          }}>
            {details}
          </pre>
        )}
      </div>
      
      <a 
        href="/hf-debug" 
        style={{
          display: 'inline-block',
          background: '#007bff',
          color: 'white',
          padding: '10px 15px',
          borderRadius: '4px',
          textDecoration: 'none'
        }}
      >
        返回环境调试
      </a>
    </div>
  )
}