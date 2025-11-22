# ALLOWED_ORIGINS 环境变量配置指南

## 概述

本文档说明如何配置 `ALLOWED_ORIGINS` 环境变量，以控制哪些域名可以访问API。

## 当前配置

### 默认白名单

后端已内置以下默认域名（无需配置）：

**开发环境**:
- `http://localhost:3000`
- `http://localhost:5173`
- `http://127.0.0.1:3000`
- `http://127.0.0.1:5173`

**Vercel部署域名**:
- `https://web-3c7a7psvt-gyc567s-projects.vercel.app`
- `https://web-pink-omega-40.vercel.app`
- `https://web-gyc567s-projects.vercel.app`
- `https://web-7jc87z3u4-gyc567s-projects.vercel.app`
- `https://web-gyc567-gyc567s-projects.vercel.app`
- `https://web-fej4rs4y2-gyc567s-projects.vercel.app`
- `https://web-fco5upt1e-gyc567s-projects.vercel.app`
- `https://web-2ybunmaej-gyc567s-projects.vercel.app`

### 环境变量覆盖

如果设置了 `ALLOWED_ORIGINS` 环境变量，它将**覆盖**默认白名单。

## 配置方法

### Replit 部署

1. 打开 Replit 项目
2. 点击左侧 **Secrets** (🔒)
3. 点击 **+ New Secret**
4. 填写：
   - **Key**: `ALLOWED_ORIGINS`
   - **Value**: 见下方"推荐值"
5. 点击 **Add secret**

### Docker 部署

```bash
# 方法1: 命令行参数
docker run -e ALLOWED_ORIGINS="https://web-pink-omega-40.vercel.app,https://web-3c7a7psvt-gyc567s-projects.vercel.app" nofx

# 方法2: 环境变量文件
echo "ALLOWED_ORIGINS=https://web-pink-omega-40.vercel.app,https://web-3c7a7psvt-gyc567s-projects.vercel.app" > .env
docker run --env-file .env nofx
```

### Systemd 部署

编辑服务文件 `/etc/systemd/system/nofx.service`:

```ini
[Service]
Environment=ALLOWED_ORIGINS=https://web-pink-omega-40.vercel.app,https://web-3c7a7psvt-gyc567s-projects.vercel.app

# 重载并重启
sudo systemctl daemon-reload
sudo systemctl restart nofx
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nofx-backend
spec:
  template:
    spec:
      containers:
      - name: nofx
        image: nofx:latest
        env:
        - name: ALLOWED_ORIGINS
          value: "https://web-pink-omega-40.vercel.app,https://web-3c7a7psvt-gyc567s-projects.vercel.app"
```

## 推荐值

### 开发环境

```bash
# 无需设置，使用默认值即可
# 默认已包含所有常用开发域名
```

### 生产环境

```bash
# 基础配置
ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app

# 完整配置
ALLOWED_ORIGINS=https://web-3c7a7psvt-gyc567s-projects.vercel.app,https://web-pink-omega-40.vercel.app,https://web-gyc567s-projects.vercel.app,https://web-7jc87z3u4-gyc567s-projects.vercel.app,https://web-gyc567-gyc567s-projects.vercel.app
```

### 特殊场景

如果需要支持所有Vercel域名（不推荐，安全性较低）:

```bash
# ⚠️ 谨慎使用，仅测试环境
ALLOWED_ORIGINS=*.vercel.app
```

## 验证配置

### 检查环境变量

```bash
# 检查是否设置
echo $ALLOWED_ORIGINS

# 应该输出逗号分隔的域名列表
```

### 测试CORS

```bash
# 测试允许的域名
curl -H "Origin: https://web-pink-omega-40.vercel.app" \
     -H "Access-Control-Request-Method: GET" \
     -X OPTIONS https://nofx-gyc567.replit.app/api/competition \
     -I

# 预期响应:
# HTTP/1.1 200 OK
# Access-Control-Allow-Origin: https://web-pink-omega-40.vercel.app
# Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
```

### 查看日志

```bash
# 查看CORS相关日志
grep -i cors /var/log/nofx-backend.log

# 或在Replit控制台查看
```

## 域名管理

### 添加新域名

1. 获取新的Vercel域名
2. 更新环境变量：

```bash
# 旧值
ALLOWED_ORIGINS=https://domain1.vercel.app

# 新值
ALLOWED_ORIGINS=https://domain1.vercel.app,https://domain2.vercel.app
```

3. 重启服务

### 移除域名

1. 从环境变量中删除域名
2. 重启服务

### 批量管理

对于多个域名，建议：

```bash
# 使用多行（某些平台支持）
ALLOWED_ORIGINS="
https://domain1.vercel.app,
https://domain2.vercel.app,
https://domain3.vercel.app
"
```

## 故障排除

### 问题1: CORS仍然被阻止

**可能原因**:
- 环境变量未生效
- 域名拼写错误
- 服务未重启

**解决方法**:
1. 检查环境变量是否正确设置
2. 验证域名拼写（注意https://前缀）
3. 重启服务

### 问题2: 所有域名都被拒绝

**可能原因**:
- 环境变量格式错误
- 包含非法字符

**解决方法**:
1. 确保域名用逗号分隔，无空格
2. 检查是否有特殊字符

### 问题3: 开发环境无法访问

**可能原因**:
- 环境变量覆盖了默认值

**解决方法**:
1. 在环境变量中添加开发域名
2. 或删除环境变量使用默认配置

## 安全建议

1. **最小权限**: 只添加必需的域名
2. **定期审查**: 清理未使用的域名
3. **监控日志**: 关注CORS拒绝请求
4. **避免通配符**: 不使用 `*` 除非必要

## 相关文档

- [CORS白名单扩展提案](../web/openspec/changes/fix-cors-allow-vercel-domains/proposal.md)
- [CORS配置技术规范](../web/openspec/changes/fix-cors-allow-vercel-domains/specs/cors-config-spec.md)
- [P0认证修复报告](P0_AUTH_FIX_SUMMARY.md)

---

**文档版本**: v1.0
**创建时间**: 2025-11-22
**维护者**: 开发团队
