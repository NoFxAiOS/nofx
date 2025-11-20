// 浏览器诊断脚本
// 在 https://web-pink-omega-40.vercel.app/dashboard 页面的控制台运行

console.log('========== 开始诊断 ==========');

// 1. 检查API基础URL配置
const apiUrl = window.__VITE_API_URL__ || 'https://nofx-gyc567.replit.app/api';
console.log('API基础URL:', apiUrl);

// 2. 获取认证token
const token = localStorage.getItem('auth_token') || localStorage.getItem('token');
console.log('认证Token:', token ? '已设置 (' + token.substring(0, 30) + '...)' : '未设置');

// 3. 测试 /api/competition 接口
fetch('https://nofx-gyc567.replit.app/api/competition', {
  headers: token ? { 'Authorization': 'Bearer ' + token } : {}
})
.then(res => {
  console.log('/api/competition HTTP状态:', res.status);
  return res.json();
})
.then(data => {
  console.log('/api/competition 响应:', data);
  if (data.traders && data.traders[0]) {
    console.log('🔍 交易员 total_equity:', data.traders[0].total_equity);
  }
})
.catch(err => console.error('/api/competition 错误:', err));

// 4. 测试 /api/account 接口
fetch('https://nofx-gyc567.replit.app/api/account', {
  headers: token ? { 'Authorization': 'Bearer ' + token } : {}
})
.then(res => {
  console.log('/api/account HTTP状态:', res.status);
  return res.json();
})
.then(data => {
  console.log('/api/account 响应:', data);
  console.log('🔍 total_equity:', data.total_equity);
  console.log('🔍 available_balance:', data.available_balance);
})
.catch(err => console.error('/api/account 错误:', err));

// 5. 检查页面上的React状态（如果可用）
setTimeout(() => {
  console.log('========== 页面状态检查 ==========');
  // 尝试从页面元素获取显示的值
  const statCards = document.querySelectorAll('[class*="stat"]');
  console.log('找到统计卡片数量:', statCards.length);

  // 检查是否有CORS错误
  console.log('如果看到 CORS 错误，说明后端没有允许 Vercel 域名的跨域请求');
  console.log('========== 诊断完成 ==========');
}, 2000);
