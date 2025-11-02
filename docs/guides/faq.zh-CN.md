# 常见问题

## 币安持仓模式错误 (code=-4061)

**错误信息**：`Order's position side does not match user's setting`

**原因**：系统需要使用双向持仓模式，但您的币安账户设置为单向持仓。

### 解决方法

1. 登录 [币安合约交易平台](https://www.binance.com/zh-CN/futures/BTCUSDT)

2. 点击右上角的 **⚙️ 偏好设置**

3. 选择 **持仓模式**

4. 切换为 **双向持仓** (Hedge Mode)

5. 确认切换

**注意**：切换前必须先平掉所有持仓。

---

## 如何生成币安 API 密钥

1. 访问 [币安 API 管理页面](https://www.binance.com/zh-CN/support/faq/detail/6b9a63f1e3384cf48a2eedb82767a69a)

2. 登录您的币安账户

3. 进入 **API 管理** 部分

4. 点击 **创建 API**

5. 设置 API 密钥名称（例如："NOFX 交易"）

6. 启用 **启用现货与杠杆交易** 和 **启用期货**

7. 配置 IP 访问限制（可选但推荐）

8. 点击 **创建**

9. 安全保存您的 **API Key** 和 **Secret Key**

10. 为了增强安全性，币安现在支持多种 API 密钥类型：

    - **HMAC**（默认）- 传统签名方法
    - **ED25519** - 更安全的椭圆曲线加密
    - **RSA** - RSA 公钥加密

    您可以在 NOFX 配置中使用 `binance_api_key_type` 字段指定密钥类型。

---

## 如何将多行 PEM 格式转换为单行

使用 Ed25519 或 RSA API 密钥时，币安提供的是多行 PEM 格式的密钥。要在 NOFX 中使用这些密钥，您需要将其转换为单行格式：

### 方法 1：使用 awk（Linux/macOS）

```bash
awk '{printf "%s", $0} END {print ""}' private_key.pem
```

### 方法 2：使用 Python

```python
with open('private_key.pem', 'r') as f:
    content = f.read().replace('\n', '')
    print(content)
```

### 方法 3：使用 sed（Linux/macOS）

```bash
sed ':a;N;$!ba;s/\n//g' private_key.pem
```

**注意**：处理私钥时要小心。切勿与他人分享或提交到版本控制系统中。

---

更多问题请查看 [GitHub Issues](https://github.com/tinkle-community/nofx/issues)
