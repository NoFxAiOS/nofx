// lib/supabase.js
import { createClient } from '@supabase/supabase-js'

const supabaseUrl = process.env.NEXT_PUBLIC_SUPABASE_URL
const supabaseAnonKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY

// 详细的调试信息
console.log('🔧 Supabase 客户端初始化:');
console.log('URL:', supabaseUrl ? '✅ 已设置' : '❌ 未设置');
console.log('Key 长度:', supabaseAnonKey?.length || 0);
console.log('Key 前缀:', supabaseAnonKey?.substring(0, 10) || '无');

// 检查环境变量
if (!supabaseUrl || !supabaseAnonKey) {
  console.error('❌ Supabase 环境变量缺失:');
  console.error('- NEXT_PUBLIC_SUPABASE_URL:', supabaseUrl);
  console.error('- NEXT_PUBLIC_SUPABASE_ANON_KEY 长度:', supabaseAnonKey?.length);
  
  // 创建降级客户端，但会失败
  console.warn('⚠️ 使用无效的 Supabase 客户端，连接将失败');
}

// 创建 Supabase 客户端
export const supabase = createClient(
  supabaseUrl || 'https://invalid-url.supabase.co',
  supabaseAnonKey || 'invalid-key',
  {
    auth: {
      persistSession: true,
      autoRefreshToken: true,
    },
    realtime: {
      params: {
        eventsPerSecond: 10,
      },
    },
  }
)

// 导出测试函数
export const testSupabaseConnection = async () => {
  try {
    console.log('🧪 开始测试 Supabase 连接...');
    
    if (!supabaseUrl || !supabaseAnonKey) {
      return {
        success: false,
        error: '环境变量未设置',
        details: {
          url: !!supabaseUrl,
          key: !!supabaseAnonKey
        }
      };
    }

    // 测试认证
    const { data: authData, error: authError } = await supabase.auth.getSession();
    
    if (authError) {
      return {
        success: false,
        error: `认证失败: ${authError.message}`,
        details: authError
      };
    }

    // 测试数据库查询（尝试读取用户表）
    const { data, error } = await supabase
      .from('users')
      .select('count')
      .limit(1);

    if (error) {
      return {
        success: false,
        error: `数据库查询失败: ${error.message}`,
        details: error,
        auth: authData
      };
    }

    return {
      success: true,
      message: 'Supabase 连接成功！',
      details: {
        auth: authData,
        query: data
      }
    };

  } catch (err) {
    return {
      success: false,
      error: `连接异常: ${err.message}`,
      details: err
    };
  }
};