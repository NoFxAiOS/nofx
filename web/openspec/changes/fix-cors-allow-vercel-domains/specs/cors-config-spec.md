# CORS白名单扩展技术规范

## 1. 概述

本文档详细说明如何扩展CORS白名单以支持所有Vercel部署域名，修复当前数据显示异常问题。

## 2. 问题分析

### 2.1 当前配置

**位置**: `api/server.go:52-99`

**当前代码**:
```go
func corsMiddleware() gin.HandlerFunc {
    allowedOrigins := []string{
        "http://localhost:3000",
        "http://localhost:5173",
        "http://127.0.0.1:3000",
        "http://127.0.0.1:5173",
    }

    // 如果设置了环境变量，使用环境变量中的值
    if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
        allowedOrigins = strings.Split(envOrigins, ",")
        for i := range allowedOrigins {
            allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
        }
    }

    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")

        // 检查origin是否在白名单中
        allowed := false
        for _, allowedOrigin := range allowedOrigins {
            if origin == allowedOrigin {
                allowed = true
                break
            }
        }

        // 仅对白名单域名设置CORS头
        if allowed {
            c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
            c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        }

        // ... 其他设置 ...
    }
}
```

### 2.2 问题识别

**缺失的域名**:
- `https://web-pink-omega-40.vercel.app`
- `https://web-gyc567s-projects.vercel.app`
- `https://web-7jc87z3u4-gyc567s-projects.vercel.app`
- `https://web-gyc567-gyc567s-projects.vercel.app`

**影响**:
- CORS预检请求失败
- API调用被拒绝
- 前端数据无法加载

## 3. 解决方案

### 3.1 更新默认白名单

**修改方案**:
```go
func corsMiddleware() gin.HandlerFunc {
    // 从环境变量获取允许的域名列表，默认为开发环境和Vercel域名
    allowedOrigins := []string{
        // 开发环境
        "http://localhost:3000",
        "http://localhost:5173",
        "http://127.0.0.1:3000",
        "http://127.0.0.1:5173",

        // Vercel部署域名 - 主要实例
        "https://web-3c7a7psvt-gyc567s-projects.vercel.app",
        "https://web-pink-omega-40.vercel.app",
        "https://web-gyc567s-projects.vercel.app",
        "https://web-7jc87z3u4-gyc567s-projects.vercel.app",
        "https://web-gyc567-gyc567s-projects.vercel.app",

        // Vercel部署域名 - 历史实例
        "https://web-fej4rs4y2-gyc567s-projects.vercel.app",
        "https://web-fco5upt1e-gyc567s-projects.vercel.app",
        "https://web-2ybunmaej-gyc567s-projects.vercel.app",
        "https://web-7jc87z3u4-gyc567s-projects.vercel.app",
    }

    // 如果设置了环境变量，使用环境变量中的值
    if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
        allowedOrigins = strings.Split(envOrigins, ",")
        for i := range allowedOrigins {
            allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
        }
    }

    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")

        // 检查origin是否在白名单中
        allowed := false
        for _, allowedOrigin := range allowedOrigins {
            if origin == allowedOrigin {
                allowed = true
                break
            }
        }

        // 仅对白名单域名设置CORS头
        if allowed {
            c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
            c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        }

        c.Writer.Header().Set("Access-Control-Allow-Methods",
            "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers",
            "Content-Type, Authorization, Cache-Control, X-Requested-With, X-Requested-By, If-Modified-Since, Pragma")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(http.StatusOK)
            return
        }

        c.Next()
    }
}
```

### 3.2 环境变量配置

#### 生产环境

```bash
# 方式1: 单行配置
export ALLOWED_ORIGINS="https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app,https://web-gyc567s-projects.vercel.app,https://web-7jc87z3u4-gyc567s-projects.vercel.app"

# 方式2: 多行配置（更易读）
export ALLOWED_ORIGINS="
https://web-3c7a7psvt-gyc567s-projects.vercel.app,
https://web-pink-omega-40.vercel.app,
https://web-gyc567s-projects.vercel.app,
https://web-7jc87z3u4-gyc567s-projects.vercel.app,
https://web-gyc567-gyc567s-projects.vercel.app"
```

#### Vercel部署 (可选)

如果后端也部署到Vercel，使用环境变量：
```bash
# 在 Vercel 项目设置中添加环境变量
ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app
```

### 3.3 域名管理策略

#### 当前域名清单

| 域名 | 状态 | 用途 |
|------|------|------|
| `web-3c7a7psvt-gyc567s-projects.vercel.app` | ✅ 活跃 | Dashboard页面 |
| `web-pink-omega-40.vercel.app` | ✅ 活跃 | Competition页面 |
| `web-gyc567s-projects.vercel.app` | ✅ 活跃 | 通用部署 |
| `web-7jc87z3u4-gyc567s-projects.vercel.app` | ✅ 活跃 | 测试实例 |
| `web-gyc567-gyc567s-projects.vercel.app` | ✅ 活跃 | 历史实例 |

#### 域名命名模式

Vercel域名格式：`https://web-[hash]-[project].vercel.app`

**规则**：
- `[hash]`: 8字符随机字符串
- `[project]`: 项目名称（gyc567s-projects）
- 所有域名都遵循此模式

#### 未来域名添加

当部署新Vercel实例时：

1. **临时方案**: 手动添加到环境变量
2. **长期方案**: 使用通配符支持（可选）

**通配符支持**（高级）:
```go
func corsMiddleware() gin.HandlerFunc {
    allowedOrigins := []string{
        // 开发环境
        "http://localhost:3000",
        "http://localhost:5173",

        // 支持 *.vercel.app 通配符
        "*.vercel.app",
    }

    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")

        allowed := false
        for _, allowedOrigin := range allowedOrigins {
            if allowedOrigin == origin {
                allowed = true
                break
            }
            // 通配符匹配
            if strings.Contains(allowedOrigin, "*") {
                pattern := strings.ReplaceAll(allowedOrigin, "*", ".*")
                matched, _ := regexp.MatchString(pattern, origin)
                if matched {
                    allowed = true
                    break
                }
            }
        }

        if allowed {
            c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
            c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        }
        // ...
    }
}
```

**注意**: 通配符方案安全性较低，仅建议在受控环境下使用。

## 4. 测试规范

### 4.1 单元测试

**测试用例**:
```go
func TestCORSAllowedOrigins(t *testing.T) {
    // 测试本地开发域名
    allowedOrigins := []string{
        "http://localhost:3000",
        "https://web-pink-omega-40.vercel.app",
    }

    tests := []struct {
        origin      string
        expected    bool
        description string
    }{
        {"http://localhost:3000", true, "本地开发域名应该被允许"},
        {"https://web-pink-omega-40.vercel.app", true, "Vercel域名应该被允许"},
        {"https://evil.com", false, "未知域名应该被拒绝"},
        {"", false, "空origin应该被拒绝"},
    }

    for _, test := range tests {
        t.Run(test.description, func(t *testing.T) {
            allowed := false
            for _, allowedOrigin := range allowedOrigins {
                if test.origin == allowedOrigin {
                    allowed = true
                    break
                }
            }
            if allowed != test.expected {
                t.Errorf("Origin %s: expected %v, got %v",
                    test.origin, test.expected, allowed)
            }
        })
    }
}
```

### 4.2 集成测试

#### 测试场景1: Vercel域名访问

```bash
# 测试 web-pink-omega-40.vercel.app
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Access-Control-Request-Method: GET" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -v

# 预期响应:
# HTTP/1.1 200 OK
# Access-Control-Allow-Origin: https://web-pink-omega-40.vercel.app
# Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
# Access-Control-Allow-Headers: Content-Type, Authorization, ...
```

#### 测试场景2: 未知域名访问

```bash
# 测试恶意域名
curl -H "Origin: https://evil.com" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -v

# 预期响应:
# HTTP/1.1 200 OK
# 注意: 不包含 Access-Control-Allow-Origin 头
```

#### 测试场景3: 实际API调用

```bash
# 测试GET请求
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Authorization: Bearer <token>" \
     https://nofx-gyc567.replit.app/api/competition

# 预期: 返回JSON数据，无CORS错误
```

### 4.3 浏览器测试

#### 测试步骤

1. **打开浏览器** (Chrome/Firefox)
2. **访问测试页面**:
   - `https://web-pink-omega-40.vercel.app/competition`
3. **打开开发者工具** (F12)
4. **检查Network面板**:
   - 所有API请求应该成功（200/OK）
   - 无CORS错误
   - Response包含正确数据
5. **检查Console面板**:
   - 无CORS相关错误
   - 无网络错误

#### 预期结果

- ✅ API请求成功
- ✅ 数据正常加载
- ✅ 无CORS错误信息
- ✅ 页面功能正常

## 5. 部署规范

### 5.1 环境变量设置

#### 开发环境

```bash
# 无需设置，使用默认值
# 默认包含本地开发域名和主要Vercel域名
```

#### 生产环境 (Replit)

```bash
# 在 Secrets 选项卡中添加
ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app
```

#### 生产环境 (其他平台)

```bash
# Docker
docker run -e ALLOWED_ORIGINS="https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app" nofx

# Systemd
Environment=ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app

# Kubernetes
env:
- name: ALLOWED_ORIGINS
  value: "https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app"
```

### 5.2 配置验证

#### 验证环境变量

```bash
# 检查环境变量
echo $ALLOWED_ORIGINS

# 应该输出所有域名，逗号分隔
```

#### 验证CORS配置

```bash
# 测试所有已知域名
for domain in "https://web-3c7a7psvt-gyc567s-projects.vercel.app" \
             "https://web-pink-omega-40.vercel.app" \
             "https://web-gyc567s-projects.vercel.app"; do
    echo "Testing $domain..."
    curl -H "Origin: $domain" -X OPTIONS https://nofx-gyc567.replit.app/api/competition -I | grep "Access-Control"
done
```

### 5.3 回滚方案

#### 快速回滚

```bash
# 方案1: 恢复CORS为通配符（仅开发/测试）
# 编辑 api/server.go
c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

# 方案2: 使用默认域名列表
# 删除环境变量 ALLOWED_ORIGINS
unset ALLOWED_ORIGINS

# 重新部署
go build -o nofx-backend ./main.go
```

#### 监控指标

- CORS拒绝请求数: `grep "CORS" /var/log/nofx.log | wc -l`
- API成功率: 监控API端点返回状态
- 错误率: 检查500错误和超时

## 6. 安全考虑

### 6.1 域名管理

**最佳实践**:
1. 仅添加活跃的、生产环境域名
2. 定期清理未使用的域名
3. 使用环境变量而非硬编码
4. 监控未授权访问尝试

**禁止**:
- ❌ 使用 `*` 通配符（除非开发环境）
- ❌ 添加测试域名到生产环境
- ❌ 忽略未知域名的CORS请求

### 6.2 监控与告警

**监控项**:
- CORS拒绝次数
- 拒绝的域名列表
- API响应时间
- 错误率

**告警条件**:
- CORS拒绝次数 > 100/小时
- 出现未知域名尝试访问
- API错误率 > 5%

### 6.3 合规性

**遵守标准**:
- OWASP CORS安全指南
- MDN CORS最佳实践
- 内部安全政策

## 7. 文档维护

### 7.1 域名清单

每次部署新Vercel实例时，更新以下文档：

1. 本技术规范文档
2. `proposal.md` 中的域名列表
3. 环境变量配置文档

### 7.2 变更记录

| 日期 | 变更 | 作者 |
|------|------|------|
| 2025-11-22 | 初始版本，添加主要Vercel域名 | Claude Code |

## 8. 参考资料

- [MDN CORS指南](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
- [OWASP CORS安全](https://owasp.org/www-community/controls/CORS_Configuration_Cheat_Sheet)
- [Gin CORS中间件](https://github.com/gin-contrib/cors)
- [P0认证修复报告](../../fix-p0-auth-issues/P0_AUTH_FIX_SUMMARY.md)

---

**文档版本**: v1.0
**维护者**: 开发团队
**审核人**: 安全团队
**最后更新**: 2025-11-22
