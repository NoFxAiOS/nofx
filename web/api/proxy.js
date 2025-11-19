/**
 * Vercel Edge Function - API代理
 * 用于解决Vercel部署保护机制阻止API访问的问题
 * 将前端API请求代理到后端，绕过部署保护
 *
 * 适用于: Vite/纯静态项目
 * 使用标准Web API而非Next.js API
 */

// 获取后端API URL
function getBackendUrl() {
  // 优先使用环境变量，否则使用默认URL
  return process.env.VITE_API_URL || 'https://nofx-gyc567.replit.app';
}

export default async function handler(req) {
  try {
    console.log('[API Proxy] 收到请求:', req.method, req.url);

    // 解析URL
    const url = new URL(req.url);
    const path = url.pathname;
    const search = url.search;

    // 构建目标URL
    const backendUrl = getBackendUrl();
    const targetUrl = `${backendUrl}${path}${search}`;

    console.log(`[API Proxy] 代理到: ${targetUrl}`);

    // 准备请求头
    const headers = new Headers();
    const excludedHeaders = [
      'host',
      'connection',
      'keep-alive',
      'proxy-authenticate',
      'proxy-authorization',
      'te',
      'trailers',
      'transfer-encoding',
      'upgrade',
      'content-length',
    ];

    // 复制非排除的请求头
    for (const [key, value] of req.headers.entries()) {
      if (!excludedHeaders.includes(key.toLowerCase())) {
        headers.set(key, value);
      }
    }

    // 添加转发信息
    headers.set('X-Forwarded-For', req.headers.get('x-forwarded-for') || 'unknown');
    headers.set('X-Forwarded-Proto', url.protocol.replace(':', ''));

    // 构建请求配置
    const requestConfig = {
      method: req.method,
      headers: headers,
    };

    // 添加请求体（对于POST/PUT/PATCH请求）
    if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(req.method)) {
      try {
        const body = await req.text();
        requestConfig.body = body;
        headers.set('Content-Length', body.length.toString());
      } catch (error) {
        console.warn('[API Proxy] 无法读取请求体:', error);
      }
    }

    console.log(`[API Proxy] 发送请求到后端...`);

    // 执行代理请求
    const response = await fetch(targetUrl, requestConfig);

    console.log(`[API Proxy] 后端响应: ${response.status} ${response.statusText}`);

    // 获取响应头
    const responseHeaders = new Headers();
    const excludedResponseHeaders = [
      'connection',
      'keep-alive',
      'proxy-authenticate',
      'proxy-authorization',
      'te',
      'trailers',
      'transfer-encoding',
      'upgrade',
      'content-length',
    ];

    // 复制响应头（排除连接相关头）
    for (const [key, value] of response.headers.entries()) {
      if (!excludedResponseHeaders.includes(key.toLowerCase())) {
        responseHeaders.set(key, value);
      }
    }

    // 获取响应体
    const responseText = await response.text();

    console.log(`[API Proxy] 返回响应给客户端: ${response.status}`);

    // 返回响应
    return new Response(responseText, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });

  } catch (error) {
    console.error('[API Proxy] 错误:', error);

    // 返回错误响应
    const errorResponse = {
      error: 'API代理失败',
      message: error.message,
      timestamp: new Date().toISOString(),
    };

    return new Response(JSON.stringify(errorResponse), {
      status: 500,
      headers: {
        'Content-Type': 'application/json',
        'Access-Control-Allow-Origin': '*',
      },
    });
  }
}
