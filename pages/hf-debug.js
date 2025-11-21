export default function HfDebug() {
  return (
    <div style={{ padding: '20px', fontFamily: 'Arial, sans-serif' }}>
      <h1>🔧 Hugging Face Space 环境调试</h1>
      
      <div style={{ background: '#f0f8ff', padding: '15px', borderRadius: '8px', marginBottom: '20px' }}>
        <h2>环境变量状态</h2>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: '#e6f3ff' }}>
              <th style={{ padding: '8px', border: '1px solid #ccc', textAlign: 'left' }}>变量名</th>
              <th style={{ padding: '8px', border: '1px solid #ccc', textAlign: 'left' }}>状态</th>
              <th style={{ padding: '8px', border: '1px solid #ccc', textAlign: 'left' }}>值预览</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td style={{ padding: '8px', border: '1px solid #ccc' }}>NEXT_PUBLIC_SUPABASE_URL</td>
              <td style={{ padding: '8px', border: '1px solid #ccc', color: process.env.NEXT_PUBLIC_SUPABASE_URL ? 'green' : 'red' }}>
                {process.env.NEXT_PUBLIC_SUPABASE_URL ? '✅ 已设置' : '❌ 未设置'}
              </td>
              <td style={{ padding: '8px', border: '1px solid #ccc', fontFamily: 'monospace' }}>
                {process.env.NEXT_PUBLIC_SUPABASE_URL || '-'}
              </td>
            </tr>
            <tr>
              <td style={{ padding: '8px', border: '1px solid #ccc' }}>NEXT_PUBLIC_SUPABASE_ANON_KEY</td>
              <td style={{ padding: '8px', border: '1px solid #ccc', color: process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ? 'green' : 'red' }}>
                {process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ? '✅ 已设置' : '❌ 未设置'}
              </td>
              <td style={{ padding: '8px', border: '1px solid #ccc', fontFamily: 'monospace' }}>
                {process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ? 
                  `${process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY.substring(0, 20)}... (${process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY.length} 字符)` : 
                  '-'
                }
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div style={{ background: '#fff3cd', padding: '15px', borderRadius: '8px' }}>
        <h3>📋 检查清单</h3>
        <ul>
          <li>{process.env.NEXT_PUBLIC_SUPABASE_URL ? '✅' : '❌'} SUPABASE_URL 已设置</li>
          <li>{process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ? '✅' : '❌'} SUPABASE_ANON_KEY 已设置</li>
          <li>{process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY?.startsWith('sbp_') ? '✅' : '❌'} 密钥格式正确 (以 sbp_ 开头)</li>
          <li>{process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY?.length === 51 || process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY?.length === 52 ? '✅' : '❌'} 密钥长度正确</li>
        </ul>
      </div>

      {process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY && (
        <div style={{ marginTop: '20px' }}>
          <a 
            href="/test-connection" 
            style={{
              display: 'inline-block',
              background: '#28a745',
              color: 'white',
              padding: '10px 15px',
              borderRadius: '4px',
              textDecoration: 'none'
            }}
          >
            测试 Supabase 连接
          </a>
        </div>
      )}
    </div>
  )
}