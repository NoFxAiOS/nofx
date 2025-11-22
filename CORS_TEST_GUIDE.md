# CORS配置测试指南

## 测试概述

本指南用于验证CORS白名单扩展是否正确配置，确保所有Vercel域名能够正常访问API。

## 快速验证

### 1. 命令行测试

```bash
# 测试 web-pink-omega-40.vercel.app (主要问题域名)
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Access-Control-Request-Method: GET" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -I

# 预期结果:
# HTTP/1.1 200 OK
# Access-Control-Allow-Origin: https://web-pink-omega-40.vercel.app
# Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
```

### 2. 浏览器测试

访问以下页面，检查是否正常加载数据：

- ✅ `https://web-pink-omega-40.vercel.app/competition`
- ✅ `https://web-3c7a7psvt-gyc567s-projects.vercel.app/dashboard`

**检查项**:
- [ ] 页面正常加载
- [ ] 数据正常显示
- [ ] 开发者工具 → Network → 无CORS错误
- [ ] 开发者工具 → Console → 无错误信息

## 详细测试用例

### 测试用例1: 允许的域名

```bash
#!/bin/bash
# 测试所有允许的域名

domains=(
    "https://web-3c7a7psvt-gyc567s-projects.vercel.app"
    "https://web-pink-omega-40.vercel.app"
    "https://web-gyc567s-projects.vercel.app"
    "https://web-7jc87z3u4-gyc567s-projects.vercel.app"
    "https://web-gyc567-gyc567s-projects.vercel.app"
)

for domain in "${domains[@]}"; do
    echo "测试域名: $domain"
    response=$(curl -H "Origin: $domain" \
                    -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
                    -I 2>/dev/null | grep "Access-Control-Allow-Origin")
    
    if [[ -n "$response" ]]; then
        echo "  ✅ 通过: $response"
    else
        echo "  ❌ 失败: 未找到CORS头"
    fi
    echo
done
```

### 测试用例2: 拒绝的域名

```bash
# 测试恶意域名
curl -H "Origin: https://evil.com" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -I

# 预期: 无 Access-Control-Allow-Origin 头
```

### 测试用例3: 实际API调用

```bash
# 获取竞赛数据
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Authorization: Bearer <token>" \
     https://nofx-gyc567.replit.app/api/competition

# 预期: 返回JSON数据，无CORS错误
```

### 测试用例4: 本地开发环境

```bash
# 测试本地开发域名
curl -H "Origin: http://localhost:3000" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -I

# 预期: 返回CORS头
```

## 前端测试步骤

### Chrome浏览器测试

1. 打开 `https://web-pink-omega-40.vercel.app/competition`
2. 按 `F12` 打开开发者工具
3. 切换到 **Network** 标签
4. 刷新页面 (`Ctrl+R`)
5. 检查请求：
   - [ ] `/api/competition` 状态码: 200
   - [ ] 响应包含JSON数据
   - [ ] 无CORS错误
6. 切换到 **Console** 标签
7. 检查是否有错误：
   - [ ] 无 `CORS` 错误
   - [ ] 无网络错误

### Firefox浏览器测试

1. 打开 `https://web-pink-omega-40.vercel.app/competition`
2. 按 `F12` 打开开发者工具
3. 切换到 **网络** 标签
4. 刷新页面
5. 查找 `/api/competition` 请求
6. 检查响应头：
   - [ ] 状态码: 200
   - [ ] 响应内容: JSON数据
   - [ ] 响应头包含: `Access-Control-Allow-Origin`

### 移动端测试

1. 使用手机浏览器访问
2. 检查页面加载
3. 检查数据显示

## 自动化测试脚本

### Node.js测试脚本

```javascript
const testCors = async () => {
    const domains = [
        'https://web-pink-omega-40.vercel.app',
        'https://web-3c7a7psvt-gyc567s-projects.vercel.app',
    ];

    for (const domain of domains) {
        try {
            const response = await fetch('https://nofx-gyc567.replit.app/api/competition', {
                method: 'OPTIONS',
                headers: {
                    'Origin': domain,
                    'Access-Control-Request-Method': 'GET',
                },
            });

            const corsHeader = response.headers.get('Access-Control-Allow-Origin');
            
            console.log(`域名: ${domain}`);
            console.log(`状态: ${response.status}`);
            console.log(`CORS头: ${corsHeader}`);
            console.log('---');
        } catch (error) {
            console.error(`测试失败: ${domain}`, error);
        }
    }
};

testCors();
```

### Python测试脚本

```python
import requests

domains = [
    'https://web-pink-omega-40.vercel.app',
    'https://web-3c7a7psvt-gyc567s-projects.vercel.app',
]

for domain in domains:
    try:
        response = requests.options(
            'https://nofx-gyc567.replit.app/api/competition',
            headers={'Origin': domain}
        )
        
        cors_header = response.headers.get('Access-Control-Allow-Origin')
        
        print(f"域名: {domain}")
        print(f"状态: {response.status_code}")
        print(f"CORS头: {cors_header}")
        print('---')
    except Exception as e:
        print(f"测试失败: {domain} - {e}")
```

## 性能测试

### 并发测试

```bash
# 使用ab测试并发请求
ab -n 100 -c 10 -H "Origin: https://web-pink-omega-40.vercel.app" \
   https://nofx-gyc567.replit.app/api/competition

# 检查响应时间和成功率
```

### 负载测试

使用 `wrk` 或 `hey` 工具进行负载测试：

```bash
# 安装hey
go install github.com/rakyll/hey@latest

# 运行负载测试
hey -n 1000 -c 10 \
    -H "Origin: https://web-pink-omega-40.vercel.app" \
    https://nofx-gyc567.replit.app/api/competition
```

## 问题排查

### 问题1: CORS头缺失

**症状**: 响应中无 `Access-Control-Allow-Origin` 头

**可能原因**:
- 域名不在白名单中
- 环境变量配置错误
- 代码未更新

**解决方法**:
1. 检查域名是否在白名单中
2. 验证环境变量设置
3. 确认代码已更新并重启服务

### 问题2: 预检请求失败

**症状**: OPTIONS请求返回错误

**解决方法**:
```bash
# 检查预检请求
curl -v -X OPTIONS \
     -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Access-Control-Request-Method: GET" \
     https://nofx-gyc567.replit.app/api/competition
```

### 问题3: 开发环境无法访问

**症状**: 本地开发时CORS被拒绝

**解决方法**:
1. 确认环境变量包含开发域名
2. 或删除环境变量使用默认配置
3. 重启开发服务器

## 监控与告警

### 日志监控

```bash
# 查看CORS相关日志
tail -f /var/log/nofx-backend.log | grep -i cors

# 统计CORS拒绝次数
grep -i "cors.*denied" /var/log/nofx-backend.log | wc -l
```

### 告警设置

**监控指标**:
- CORS拒绝请求数
- 未知域名访问尝试
- API响应时间

**告警条件**:
- CORS拒绝 > 100次/小时
- 出现未知域名
- API错误率 > 5%

## 测试报告模板

### 测试结果记录

| 域名 | 状态码 | CORS头 | 结果 | 备注 |
|------|--------|--------|------|------|
| web-pink-omega-40.vercel.app | 200 | ✅ | 通过 | - |
| web-3c7a7psvt-gyc567s-projects.vercel.app | 200 | ✅ | 通过 | - |
| evil.com | 200 | ❌ | 通过 | 正确拒绝 |

### 问题记录

| 问题 | 影响 | 状态 | 解决方案 |
|------|------|------|----------|
| - | - | - | - |

### 总结

- **测试日期**: 2025-11-22
- **测试环境**: 生产环境
- **测试结果**: ✅ 全部通过
- **建议**: 无

---

**文档版本**: v1.0
**创建时间**: 2025-11-22
**维护者**: QA团队
