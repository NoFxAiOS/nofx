# 🔍 API接口测试工具使用说明

## 目的

测试所有前端API接口，找出主页显示0而详情页显示正确的原因。

## 步骤

### 1. 获取认证Token

打开浏览器，访问：https://web-pink-omega-40.vercel.app/dashboard

1. 按 `F12` 打开开发者工具
2. 切换到 `Console` 标签页
3. 输入以下命令：
   ```javascript
   localStorage.getItem('token')
   ```
4. 复制输出的token字符串（不包括引号）

### 2. 修改测试工具

编辑 `test_all_apis.go`：

```bash
vim test_all_apis.go
```

找到第9行：
```go
authToken = "YOUR_AUTH_TOKEN_HERE"
```

替换为你复制的token：
```go
authToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...."
```

### 3. 运行测试

```bash
cd /Users/guoyingcheng/dreame/code/nofx
go run test_all_apis.go
```

## 预期结果

测试会输出4个接口的数据：

1. **`/api/competition`** - 竞赛模式汇总
   - 如果显示 `total_equity: 0`，说明这里有问题 ❌
   - 如果显示 `total_equity: 99.92`，说明这里正常 ✅

2. **`/api/account`** - 账户汇总
   - 如果显示 `total_equity: 0`，说明这里有问题 ❌
   - 如果显示 `total_equity: 99.92`，说明这里正常 ✅

3. **`/api/my-traders`** - 交易员列表
   - 查看列表中交易员的 `total_equity` 是否正确

4. **`/api/my-traders/{id}`** - 交易员详情
   - 根据你的描述，这里应该显示正确（99.92） ✅

## 问题分析

根据测试结果，我们能判断：

| 接口 | 状态 | 可能原因 |
|-----|------|---------|
| `/api/competition` 显示0 | ❌ | 竞赛模式汇总逻辑有问题 |
| `/api/account` 显示0 | ❌ | 账户汇总逻辑有问题 |
| `/api/my-traders` 显示0 | ❌ | 列表汇总逻辑有问题 |
| `/api/my-traders/{id}` 显示正确 | ✅ | 单个交易员逻辑已修复 |

## 下一步

根据哪个接口显示0，我们会：

1. 找到对应的后端handler函数
2. 检查是否有类似的字段映射错误
3. 修复并验证
