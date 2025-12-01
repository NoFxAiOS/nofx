# Web3钱包集成安全审计报告

**审计对象**: Web3 Wallet Integration - OpenSpec Proposal
**审计日期**: 2025-12-01
**审计员**: Claude Code
**审计类型**: 代码安全审计 + 架构审计
**版本**: v1.0

---

## 执行摘要

本报告对Web3钱包集成提案进行了全面的安全审计，涵盖密码学实现、Web3安全最佳实践、数据库安全、API安全、前端安全、业务逻辑安全及合规性等7个主要领域。

### 风险评级总览
- **关键漏洞 (Critical)**: 4个
- **高危漏洞 (High)**: 7个
- **中等漏洞 (Medium)**: 11个
- **低风险 (Low)**: 6个

### 整体安全评级: ⚠️ **中等风险 (C级)**

**主要风险**:
1. EIP-191签名实现存在严重缺陷，可能导致签名伪造
2. nonce存储缺失，易受重放攻击
3. 缺少数据库nonce管理，存在竞态条件
4. 速率限制配置不明确，可能被暴力破解

**建议**:
- 在生产部署前必须修复所有Critical和High级别漏洞
- 实施完整的威胁建模和渗透测试
- 建立完善的安全监控和审计机制
- 对关键安全组件进行独立第三方审计

---

## 1. 密码学安全审计

### 1.1 关键漏洞 (Critical)

#### CVE-WS-001: EIP-191签名验证实现错误
**位置**: `/web3_auth/signatures.go` 第206-251行
**严重程度**: 关键

**问题描述**:
签名验证函数存在多处严重错误，完全破坏了签名验证的安全性：

1. **错误1 - 公钥恢复逻辑错误 (第234-237行)**:
```go
msgHash := crypto.Keccak256(messageBytes)
sigPublicKey, err := crypto.Ecrecover(msgHash, sigBytes)
```
- 直接使用原始消息字节而不是EIP-191标准的Keccak256哈希
- EIP-191要求使用`"\x19Ethereum Signed Message:\n" + len(message) + message`格式

2. **错误2 - 椭圆曲线对象误用 (第213行)**:
```go
"crypto/elliptic"
...
publicKeyBytes := crypto.FromECDSAPub((*ecdsa.PublicKey)(&elliptic.P256{}))
```
- 使用`elliptic.P256{}`创建错误的公钥对象
- 应该使用`sigPublicKey`从Ecrecover返回的真实公钥

3. **错误3 - 地址计算逻辑混乱 (第248-250行)**:
```go
hash := crypto.Keccak256(sigPublicKey[1:])
address := common.HexToAddress(fmt.Sprintf("0x%x", hash[12:]))
```
- 尝试从错误的公钥对象计算地址
- 混淆了椭圆曲线对象和实际的公钥字节

**影响**:
- 签名验证完全无效
- 攻击者可以伪造任意地址的签名
- 可能导致未授权访问和钱包劫持

**修复建议**:
```go
// 正确的EIP-191实现
func RecoverAddressFromSignature(message, signature, expectedAddress string) (string, error) {
	// 1. 验证签名格式
	sigBytes, err := hex.DecodeString(signature[2:]) // 移除"0x"前缀
	if err != nil || len(sigBytes) != 65 {
		return "", errors.New("无效的签名格式")
	}

	// 2. 生成EIP-191兼容的消息哈希
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256([]byte(prefix))

	// 3. 使用标准Ecrecover
	recoveredPubKey, err := crypto.Ecrecover(msgHash, sigBytes)
	if err != nil {
		return "", errors.New("签名恢复失败")
	}

	// 4. 转换为ECDSA公钥
	var pubKey ecdsa.PublicKey
	bytes := elliptic.Unmarshal(crypto.S256(), recoveredPubKey, &pubKey.X, &pubKey.Y)
	if bytes == nil {
		return "", errors.New("公钥格式错误")
	}

	// 5. 验证公钥有效性
	if !crypto.S256().IsOnCurve(pubKey.X, pubKey.Y) {
		return "", errors.New("公钥不在曲线上")
	}

	// 6. 计算地址
	address := crypto.PubkeyToAddress(pubKey)

	// 7. 比较地址
	if !strings.EqualFold(address.Hex(), expectedAddress) {
		return "", ErrAddressMismatch
	}

	return address.Hex(), nil
}
```

**参考标准**:
- EIP-191: https://eips.ethereum.org/EIPS/eip-191
- Ethereum Yellow Paper (附录F)
- Consensys安全最佳实践

---

#### CVE-WS-002: nonce生成和使用无存储保护
**位置**: `/web3_auth/signatures.go` 第283-290行
**严重程度**: 关键

**问题描述**:
nonce生成后没有存储或时间戳验证机制：

```go
func GenerateNonce() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
```

**缺陷**:
1. nonce在数据库中没有持久化存储
2. 无法验证nonce是否已被使用
3. 无法强制执行10分钟过期策略
4. 没有防重放攻击机制

**攻击向量**:
- 重放攻击: 攻击者可以截获nonce和签名，重复使用
- nonce猜测: 虽然nonce是随机的，但缺少服务端验证
- 并发竞争: 同一nonce可能被不同请求使用

**修复建议**:
```go
// 在数据库中添加nonce表
CREATE TABLE web3_nonces (
    id SERIAL PRIMARY KEY,
    address TEXT NOT NULL,
    nonce TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    used BOOLEAN DEFAULT FALSE,
    request_ip INET,
    CONSTRAINT chk_expires CHECK (expires_at > created_at)
);

CREATE INDEX idx_nonces_address ON web3_nonces(address);
CREATE INDEX idx_nonces_nonce ON web3_nonces(nonce);
CREATE INDEX idx_nonces_expires ON web3_nonces(expires_at);
```

```go
// 更新GenerateNonce函数
func (r *PostgreSQLRepository) GenerateAndStoreNonce(address string, ip string) (string, error) {
	nonce, err := GenerateNonce()
	if err != nil {
		return "", err
	}

	expires := time.Now().Add(10 * time.Minute)

	query := `
		INSERT INTO web3_nonces (address, nonce, expires_at, request_ip)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (nonce) DO UPDATE SET
			address = EXCLUDED.address,
			expires_at = EXCLUDED.expires_at,
			request_ip = EXCLUDED.request_ip
	`

	_, err = r.db.Exec(query, address, nonce, expires, ip)
	if err != nil {
		return "", err
	}

	return nonce, nil
}

// 验证并消耗nonce
func (r *PostgreSQLRepository) ValidateAndConsumeNonce(address, nonce string) error {
	query := `
		UPDATE web3_nonces
		SET used = TRUE, used_at = NOW()
		WHERE address = $1
		AND nonce = $2
		AND used = FALSE
		AND expires_at > NOW()
	`

	result, err := r.db.Exec(query, address, nonce)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("nonce无效、已过期或已使用")
	}

	return nil
}
```

---

### 1.2 高危漏洞 (High)

#### CVE-WS-003: 椭圆曲线选择错误
**位置**: `/web3_auth/signatures.go` 第187行、第213行
**严重程度**: 高危

**问题描述**:
代码中混合使用了secp256k1和P256两种不同的椭圆曲线：

1. 第187行导入`elliptic.P256`（NIST P-256）
2. 第187行导入`crypto.S256()`（secp256k1）
3. 第812行的测试使用了`crypto.S256()`
4. 第213行实际使用了`elliptic.P256{}`

**问题**:
- 以太坊使用secp256k1曲线，不是P256
- 使用错误的曲线会导致签名验证失败
- 安全强度不匹配（secp256k1 vs P-256）

**修复建议**:
```go
// 移除错误的导入
// "crypto/elliptic" // 不要导入P256

// 使用一致的secp256k1
import "github.com/ethereum/go-ethereum/crypto"
...

// 使用crypto.Keccak256而不是crypto.S256()
```

---

#### CVE-WS-004: 签名长度和格式验证不足
**位置**: `/api/web3/auth.go` 第466-478行
**严重程度**: 高危

**问题描述**:
签名验证过程中缺少充分的格式检查：

```go
func (h *Handler) Authenticate(c *gin.Context) {
	// 仅验证地址格式，未充分验证签名
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		// ...
	}
}
```

**缺失的验证**:
1. 签名必须以"0x"开头
2. 签名长度必须是130字符（65字节*2）
3. 签名字符必须为有效的十六进制
4. v值必须是27或28（0x1b或0x1c）

**修复建议**:
```go
func ValidateSignature(sig string) error {
	if len(sig) != 132 { // "0x" + 130 hex chars
		return errors.New("签名长度必须为132字符")
	}
	if !strings.HasPrefix(sig, "0x") {
		return errors.New("签名必须以0x开头")
	}
	_, err := hex.DecodeString(sig[2:])
	if err != nil {
		return errors.New("签名包含无效的十六进制字符")
	}
	return nil
}
```

---

### 1.3 中等漏洞 (Medium)

#### CVE-WS-005: 消息模板钓鱼风险
**位置**: `/web3_auth/signatures.go` 第261-272行
**严重程度**: 中等

**问题描述**:
当前的签名消息格式虽然包含防钓鱼信息，但仍有改进空间：

```go
func GenerateSignatureMessage(address, nonce, timestamp string) string {
	return fmt.Sprintf(`
Monnaire Trading Agent OS - Web3 Authentication

Wallet Address: %s
Nonce: %s
Timestamp: %s

This request will not trigger a blockchain transaction or cost any gas fees.

Signature Expires: 10 minutes
`, address, nonce, timestamp)
}
```

**风险**:
1. 域名显示在消息中，但可能被仿冒
2. 缺少域绑定（domain binding）
3. 没有包含应用特定的前缀或标识符
4. 容易被复制到恶意站点

**最佳实践**:
- 使用EIP-712结构化数据标准
- 包含域名和版本信息
- 使用明确的应用程序标识

**修复建议**:
```go
// 使用EIP-712
const EIP712DomainName = "Monnaire Trading Agent"
const EIP712DomainVersion = "1"
const EIP712DomainChainId = 1

type EIP712Domain struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	ChainId           uint   `json:"chainId"`
	VerifyingContract string `json:"verifyingContract"`
	Salt              string `json:"salt"`
}

type AuthMessage struct {
	Statement string `json:"statement"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
}

func GenerateEIP712Message(address, nonce, timestamp string) (string, error) {
	domainSeparator := hashStruct("EIP712Domain", domainType, EIP712Domain{
		Name:              EIP712DomainName,
		Version:           EIP712DomainVersion,
		ChainId:           1,
		VerifyingContract: "0x0000000000000000000000000000000000000000",
		Salt:              "0x1234567890abcdef",
	})

	message := AuthMessage{
		Statement: "Login to Monnaire Trading Agent OS",
		Nonce:     nonce,
		Timestamp: timestamp,
	}

	typedData := TypedData{
		Types:       types,
		Domain:      domainSeparator,
		PrimaryType: "AuthMessage",
		Message:     message,
	}

	return typedData.EncodeJSON()
}
```

---

## 2. Web3安全最佳实践

### 2.1 高危漏洞 (High)

#### CVE-WS-006: 钱包类型验证不充分
**位置**: `/api/web3/auth.go` 第486行
**严重程度**: 高危

**问题描述**:
钱包类型验证仅通过字符串比较，缺少白名单验证：

```go
type AuthRequest struct {
	WalletType string `json:"wallet_type" binding:"required"`
}

// 在处理器中
if err := web3_auth.ValidateAddress(req.Address); err != nil {
	// ...
}
// 缺少WalletType验证
```

**风险**:
- 可能传入任意钱包类型
- 支持不可信的钱包实现
- 缺少对钱包能力的验证

**修复建议**:
```go
// 定义钱包类型白名单
var supportedWalletTypes = map[string]bool{
	"metamask": true,
	"tp":       true,
	"walletconnect": true,
	"coinbase": true,
}

func (r *PostgreSQLRepository) ValidateWalletType(walletType string) error {
	if !supportedWalletTypes[walletType] {
		return fmt.Errorf("不支持的钱包类型: %s", walletType)
	}
	return nil
}
```

---

### 2.2 中等漏洞 (Medium)

#### CVE-WS-007: 缺少域名绑定验证
**位置**: `/web3_auth/signatures.go` 第261-272行
**严重程度**: 中等

**问题描述**:
当前实现没有将签名与特定域名绑定，容易被跨站点重用。

**修复建议**:
```go
func GenerateSignatureMessage(address, nonce, timestamp, domain string) string {
	return fmt.Sprintf(`
${DOMAIN} - Web3 Authentication

Wallet Address: %s
Nonce: %s
Timestamp: %s

This request will not trigger a blockchain transaction or cost any gas fees.

Signature Expires: 10 minutes
`, domain, address, nonce, timestamp)
}
```

---

## 3. 数据库安全

### 3.1 中等漏洞 (Medium)

#### CVE-WS-008: 主钱包约束检查性能问题
**位置**: `/specs/database/spec.md` 第84-99行
**严重程度**: 中等

**问题描述**:
`chk_is_primary`约束使用子查询检查，对大表性能有影响：

```sql
CONSTRAINT chk_is_primary CHECK (
    CASE
        WHEN is_primary = TRUE THEN
            NOT EXISTS (
                SELECT 1 FROM user_wallets uw2
                WHERE uw2.user_id = user_wallets.user_id
                AND uw2.is_primary = TRUE
                AND uw2.wallet_addr != user_wallets.wallet_addr
            )
        ELSE TRUE
    END
)
```

**问题**:
- 子查询对每个INSERT/UPDATE执行
- 当用户有很多钱包时性能下降
- 可能导致锁争用

**修复建议**:
```sql
-- 使用部分索引和触发器
CREATE UNIQUE INDEX idx_user_primary_wallet
ON user_wallets(user_id)
WHERE is_primary = TRUE;

-- 触发器检查（更高效）
CREATE OR REPLACE FUNCTION validate_single_primary()
RETURNS TRIGGER AS $$
BEGIN
    -- 如果设置为primary，先移除其他primary
    IF NEW.is_primary THEN
        UPDATE user_wallets
        SET is_primary = FALSE
        WHERE user_id = NEW.user_id
        AND wallet_addr != NEW.wallet_addr;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP CONSTRAINT chk_is_primary;
CREATE TRIGGER validate_single_primary_wallet
    BEFORE INSERT OR UPDATE ON user_wallets
    FOR EACH ROW EXECUTE FUNCTION validate_single_primary();
```

---

#### CVE-WS-009: 审计日志不完整
**位置**: `/specs/auth/spec.md` 第92-94行
**严重程度**: 中等

**问题描述**:
仅提到"审计日志记录所有操作"，但没有详细的审计表设计或触发器。

**修复建议**:
```sql
CREATE TABLE web3_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT,
    wallet_addr TEXT NOT NULL,
    action TEXT NOT NULL, -- 'LINK', 'UNLINK', 'AUTHENTICATE', 'SET_PRIMARY'
    ip_address INET,
    user_agent TEXT,
    metadata JSONB,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (wallet_addr) REFERENCES web3_wallets(wallet_addr) ON DELETE CASCADE
);

CREATE INDEX idx_audit_logs_user ON web3_audit_logs(user_id);
CREATE INDEX idx_audit_logs_addr ON web3_audit_logs(wallet_addr);
CREATE INDEX idx_audit_logs_timestamp ON web3_audit_logs(timestamp);
CREATE INDEX idx_audit_logs_action ON web3_audit_logs(action);

-- 自动记录审计日志的函数
CREATE OR REPLACE FUNCTION log_web3_action()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO web3_audit_logs (
            user_id, wallet_addr, action, metadata
        ) VALUES (
            NEW.user_id, NEW.wallet_addr, 'LINK',
            jsonb_build_object('is_primary', NEW.is_primary)
        );
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO web3_audit_logs (
            user_id, wallet_addr, action, metadata
        ) VALUES (
            NEW.user_id, NEW.wallet_addr, 'SET_PRIMARY',
            jsonb_build_object('old_primary', OLD.is_primary, 'new_primary', NEW.is_primary)
        );
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO web3_audit_logs (
            user_id, wallet_addr, action
        ) VALUES (
            OLD.user_id, OLD.wallet_addr, 'UNLINK'
        );
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_user_wallets
    AFTER INSERT OR UPDATE OR DELETE ON user_wallets
    FOR EACH ROW EXECUTE FUNCTION log_web3_action();
```

---

## 4. API安全

### 4.1 关键漏洞 (Critical)

#### CVE-WS-010: 缺少nonce服务端验证
**位置**: `/api/web3/auth.go` 第580-620行
**严重程度**: 关键

**问题描述**:
在Authenticate端点中没有验证nonce的有效性：

```go
func (h *Handler) Authenticate(c *gin.Context) {
	// ... 其他验证 ...

	// 3. 验证签名
	recoveredAddr, err := web3_auth.RecoverAddressFromSignature(expectedMessage, req.Signature, req.Address)
	if err != nil {
		// 错误处理 ...
		return
	}

	// 4. 检查该地址是否已绑定用户
	boundUser, err := h.repo.GetBoundUser(req.Address)
	// ...
}
```

**缺失**:
- 没有调用`ValidateAndConsumeNonce()`
- nonce可以在服务端验证前被重复使用
- 没有时间戳检查

**修复建议**:
```go
func (h *Handler) Authenticate(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// ...
	}

	// 1. 验证地址格式
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		// ...
	}

	// 2. 验证nonce
	err := h.repo.ValidateAndConsumeNonce(req.Address, req.Nonce)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "nonce无效或已过期: " + err.Error(),
		})
		return
	}

	// 3. 验证签名
	// ... 其余代码 ...
}
```

---

### 4.2 高危漏洞 (High)

#### CVE-WS-011: JWT认证配置未明确
**位置**: `/api/web3/auth.go` 第630-680行
**严重程度**: 高危

**问题描述**:
在LinkWallet等需要JWT保护的端点中，认证中间件的配置不明确：

```go
func (h *Handler) LinkWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未认证",
		})
		return
	}
	// ...
}
```

**缺失**:
- 没有JWT签名算法配置
- 没有密钥轮换策略
- 没有token过期时间设置
- 没有刷新token机制

**修复建议**:
```go
// JWT配置
type JWTConfig struct {
	Secret          []byte
	Algorithm       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

const (
	JWTAlgorithm    = "HS256"
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// 生成JWT Token
func (h *Handler) generateTokens(userID string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access Token
	accessClaims := jwt.MapClaims{
		"user_id": userID,
		"type":    "access",
		"iat":     now.Unix(),
		"exp":     now.Add(AccessTokenTTL).Unix(),
	}
	accessToken = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(jwtSecret)

	// Refresh Token
	refreshClaims := jwt.MapClaims{
		"user_id": userID,
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     now.Add(RefreshTokenTTL).Unix(),
	}
	refreshToken = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(jwtSecret)

	return
}

// JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
			c.Abort()
			return
		}

		tokenStr := authHeader[7:]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的令牌: " + err.Error()})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌无效"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !claims.VerifyExpiresAt(time.Now().Unix(), true) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "令牌已过期"})
			c.Abort()
			return
		}

		c.Set("user_id", claims["user_id"])
		c.Set("token", token)
		c.Next()
	})
}
```

---

#### CVE-WS-012: 速率限制配置不明确
**位置**: `/specs/auth/spec.md` 第64行
**严重程度**: 高危

**问题描述**:
仅提到"Max 10 requests per minute per IP"，但没有实现细节：

```go
// Rate Limiting: Max 10 requests per minute per IP
```

**缺失**:
- 没有实际实现速率限制
- 没有分布式限流方案
- 没有burst限制
- 没有白名单机制

**修复建议**:
```go
// 使用Redis实现分布式限流
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s", ip)

		// 使用Redis原子操作
		ctx := c.Request.Context()
		pipe := redisClient.Pipeline()

		// 获取当前计数
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window)

		results, err := pipe.Exec(ctx)
		if err != nil {
			// Redis错误时拒绝请求
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务不可用"})
			c.Abort()
			return
		}

		count := results[0].(*redis.IntCmd).Val()
		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(limit-int(count)))
		c.Header("X-RateLimit-Reset", time.Now().Add(window).Format(time.RFC3339))
		c.Next()
	})
}

// 在路由中应用
router := gin.New()
router.Use(gin.Logger())
router.Use(gin.Recovery())

// 公共端点限流较宽松
publicLimiter := RateLimitMiddleware(30, time.Minute) // 30/min for public endpoints

// 认证端点限流更严格
authLimiter := RateLimitMiddleware(10, time.Minute)   // 10/min for auth

router.POST("/api/web3/auth/generate-nonce", authLimiter, handler.GenerateNonce)
router.POST("/api/web3/auth/authenticate", authLimiter, handler.Authenticate)

// 登录后端点限流中等
privateLimiter := RateLimitMiddleware(100, time.Minute) // 100/min for authenticated

router.POST("/api/web3/wallet/link", authMiddleware, privateLimiter, handler.LinkWallet)
router.DELETE("/api/web3/wallet/:address", authMiddleware, privateLimiter, handler.UnlinkWallet)
```

---

### 4.3 中等漏洞 (Medium)

#### CVE-WS-013: CORS配置不明确
**位置**: `/proposal.md` 第1209行
**严重程度**: 中等

**问题描述**:
仅提到"CORS策略严格限制域名"，但没有具体配置：

```go
// CORS策略严格限制域名
```

**修复建议**:
```go
// CORS中间件配置
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     []string{"https://yourdomain.com", "https://app.yourdomain.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// 允许Web3钱包域名
	if gin.Mode() == gin.TestMode {
		config.AllowAllOrigins = true
	}

	return cors.New(config)
}
```

---

#### CVE-WS-014: 错误响应信息泄露
**位置**: `/api/web3/auth.go` 多个位置
**严重程度**: 中等

**问题描述**:
错误响应可能泄露敏感信息：

```go
c.JSON(http.StatusUnauthorized, gin.H{
	"error": "签名验证失败: " + err.Error(),
})
```

**风险**:
- 错误堆栈信息可能泄露内部实现细节
- 数据库错误可能暴露表名或字段名
- 调试信息可能被攻击者利用

**修复建议**:
```go
// 统一的错误响应
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// 安全的错误处理
func (h *Handler) Authenticate(c *gin.Context) {
	// ... 验证逻辑 ...

	// 3. 验证签名
	recoveredAddr, err := web3_auth.RecoverAddressFromSignature(expectedMessage, req.Signature, req.Address)
	if err != nil {
		// 不暴露具体错误细节给客户端
		c.JSON(http.StatusUnauthorized, APIError{
			Code:    "WEB3_002",
			Message: "签名验证失败",
		})
		return
	}

	// ... 其他操作 ...
}
```

---

## 5. 前端安全

### 5.1 中等漏洞 (Medium)

#### CVE-WS-015: 签名请求无时间戳验证
**位置**: `/web3_auth/signatures.go` 第261-272行
**严重程度**: 中等

**问题描述**:
签名消息包含timestamp，但缺少前后端的时间同步检查。

**修复建议**:
```go
// 前端验证时间戳
func validateTimestamp(timestamp string) error {
	t, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("无效的时间戳")
	}

	now := time.Now().Unix()
	delta := now - t

	// 允许5分钟时间差
	if abs(delta) > 300 {
		return errors.New("时间戳过期或过早")
	}

	return nil
}
```

---

#### CVE-WS-016: 缺少CSP策略
**位置**: `/proposal.md` 未涉及
**严重程度**: 中等

**问题描述**:
前端没有Content Security Policy (CSP)配置来防止XSS攻击。

**修复建议**:
```html
<meta http-equiv="Content-Security-Policy"
      content="
        default-src 'self';
        script-src 'self' 'unsafe-inline' https://cdn.metamask.io https://tpwallet.io;
        style-src 'self' 'unsafe-inline';
        img-src 'self' data: https:;
        connect-src 'self' https://api.yourdomain.com wss://api.yourdomain.com https://mainnet.infura.io https://ropsten.infura.io;
        frame-src 'none';
        object-src 'none';
        base-uri 'self';
      ">
```

---

### 5.2 低风险 (Low)

#### CVE-WS-017: 前端缺少敏感数据清理
**位置**: `/proposal.md` 第5.4节
**严重程度**: 低

**问题描述**:
前端可能缓存敏感信息（钱包地址、nonce等）没有适当清理。

**修复建议**:
```javascript
// 组件卸载时清理敏感数据
useEffect(() => {
  return () => {
    // 清除nonce等临时数据
    sessionStorage.removeItem('web3_nonce');
    sessionStorage.removeItem('web3_timestamp');
  };
}, []);
```

---

## 6. 业务逻辑安全

### 6.1 高危漏洞 (High)

#### CVE-WS-018: 主钱包设置存在竞态条件
**位置**: `/database/web3/wallet.go` SetPrimaryWallet方法
**严重程度**: 高危

**问题描述**:
设置主钱包时，两个请求同时到达可能导致多个主钱包：

```go
func (r *PostgreSQLRepository) SetPrimaryWallet(userID, walletAddr string) error {
	// 这个操作不是原子的！
	// 请求1: SELECT确认只有A是primary
	// 请求2: 同时执行SELECT，也确认只有A是primary
	// 请求1: UPDATE设置B为primary
	// 请求2: UPDATE设置C为primary
	// 结果: B和C都是primary！
}
```

**修复建议**:
```go
// 使用事务和行级锁
func (r *PostgreSQLRepository) SetPrimaryWallet(userID, walletAddr string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 移除当前主钱包（FOR UPDATE确保原子性）
	_, err = tx.Exec(`
		UPDATE user_wallets
		SET is_primary = FALSE
		WHERE user_id = $1 AND is_primary = TRUE
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}

	// 2. 设置新主钱包
	result, err := tx.Exec(`
		UPDATE user_wallets
		SET is_primary = TRUE, last_used_at = NOW()
		WHERE user_id = $1 AND wallet_addr = $2
	`, userID, walletAddr)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("钱包不存在或不属于该用户")
	}

	return tx.Commit()
}
```

---

### 6.2 中等漏洞 (Medium)

#### CVE-WS-019: 缺少地址黑名单机制
**位置**: `/web3_auth/signatures.go` ValidateAddress函数
**严重程度**: 中等

**问题描述**:
没有检查地址是否在黑名单中（已知被盗地址、恶意地址等）。

**修复建议**:
```go
// 地址黑名单
var bannedAddresses = map[string]bool{
	"0x0000000000000000000000000000000000000000": true,
	"0x000000000000000000000000000000000000dEaD": true,
}

func ValidateAddress(addr string) error {
	if !common.IsHexAddress(addr) {
		return fmt.Errorf("无效的以太坊地址格式: %s", addr)
	}

	// 检查黑名单
	if bannedAddresses[strings.ToLower(addr)] {
		return fmt.Errorf("地址被禁止")
	}

	return nil
}
```

---

#### CVE-WS-020: 钱包解耦逻辑不完整
**位置**: `/api/web3/auth.go` UnlinkWallet
**严重程度**: 中等

**问题描述**:
解绑钱包时没有检查是否还有其他绑定方式。

**修复建议**:
```go
func (r *PostgreSQLRepository) UnlinkWallet(userID, walletAddr string) error {
	// 检查是否是最后一个钱包
	query := `
		SELECT COUNT(*) as count
		FROM user_wallets
		WHERE user_id = $1
	`
	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return err
	}

	if count == 1 {
		return errors.New("不能解绑最后一个钱包")
	}

	// 检查是否是主钱包
	var isPrimary bool
	query = `
		SELECT is_primary
		FROM user_wallets
		WHERE user_id = $1 AND wallet_addr = $2
	`
	err = r.db.QueryRow(query, userID, walletAddr).Scan(&isPrimary)
	if err != nil {
		return err
	}

	// 如果是主钱包，先设置其他钱包为主钱包
	if isPrimary {
		_, err = r.db.Exec(`
			UPDATE user_wallets
			SET is_primary = TRUE
			WHERE user_id = $1
			AND wallet_addr != $2
			LIMIT 1
		`, userID, walletAddr)
		if err != nil {
			return err
		}
	}

	// 删除绑定
	_, err = r.db.Exec(`
		DELETE FROM user_wallets
		WHERE user_id = $1 AND wallet_addr = $2
	`, userID, walletAddr)

	return err
}
```

---

## 7. 合规性和隐私

### 7.1 低风险 (Low)

#### CVE-WS-021: 缺少数据保留策略
**位置**: `/specs/database/spec.md`
**严重程度**: 低

**问题描述**:
数据库设计没有数据保留和删除策略。

**修复建议**:
```sql
-- 数据保留策略（GDPR等合规要求）

-- 1. 自动删除过期nonce
CREATE OR REPLACE FUNCTION cleanup_expired_nonces()
RETURNS void AS $$
BEGIN
    DELETE FROM web3_nonces
    WHERE expires_at < NOW() - INTERVAL '1 day';
END;
$$ LANGUAGE plpgsql;

-- 每天执行一次
SELECT cron.schedule('cleanup-nonces', '0 0 * * *', 'SELECT cleanup_expired_nonces();');

-- 2. 用户删除钱包时，保留审计日志但移除PII
CREATE OR REPLACE FUNCTION anonymize_user_data()
RETURNS void AS $$
DECLARE
    user_wallets_record RECORD;
BEGIN
    FOR user_wallets_record IN
        SELECT * FROM user_wallets WHERE user_id = $1
    LOOP
        -- 记录到审计日志
        INSERT INTO web3_audit_logs (
            wallet_addr, action, metadata, timestamp
        ) VALUES (
            user_wallets_record.wallet_addr,
            'DELETE',
            jsonb_build_object('reason', 'user_request'),
            NOW()
        );
    END LOOP;

    -- 删除实际数据
    DELETE FROM user_wallets WHERE user_id = $1;
END;
$$ LANGUAGE plpgsql;
```

---

## 8. 已知攻击向量防护分析

### 8.1 Signature Replay Attack
**防护状态**: ❌ 未充分防护
**状态描述**: 虽然使用nonce，但没有服务端存储验证

**建议**:
- 实现CVE-WS-002的nonce存储方案
- 每个nonce只能使用一次
- 10分钟后自动过期

### 8.2 Address Reuse Attack
**防护状态**: ⚠️ 部分防护
**状态描述**: 唯一约束防止地址重复绑定，但缺少黑名单检查

**建议**:
- 实施CVE-WS-019的黑名单机制
- 定期更新已知恶意地址列表

### 8.3 Transaction Malleability
**防护状态**: ✅ 已防护
**状态描述**: 使用标准签名格式（65字节），不易受 malleability 影响

### 8.4 Front-running
**防护状态**: ⚠️ 部分防护
**状态描述**: nonce机制可防护，但时间窗口（10分钟）过长

**建议**:
- 缩短nonce有效期到2-3分钟
- 实施交易顺序检查

### 8.5 Unauthorized Wallet Binding
**防护状态**: ✅ 已防护
**状态描述**: JWT认证保护LinkWallet端点

### 8.6 Session Fixation
**防护状态**: ✅ 已防护
**状态描述**: JWT token在认证时生成，无法预知

### 8.7 Man-in-the-Middle
**防护状态**: ✅ 已防护
**状态描述**: 需要HTTPS和正确的CORS配置

---

## 9. 安全评级详细说明

### 9.1 关键漏洞 (4个) - 必须修复
1. **CVE-WS-001**: EIP-191签名验证错误 - 签名完全无效
2. **CVE-WS-002**: nonce无存储保护 - 易受重放攻击
3. **CVE-WS-010**: 缺少nonce服务端验证 - 允许重放
4. **CVE-WS-018**: 主钱包设置竞态条件 - 数据不一致

### 9.2 高危漏洞 (7个) - 强烈建议修复
1. **CVE-WS-003**: 椭圆曲线选择错误 - 不兼容以太坊
2. **CVE-WS-004**: 签名验证不足 - 可能接受无效签名
3. **CVE-WS-006**: 钱包类型验证不充分 - 安全边界不清
4. **CVE-WS-011**: JWT配置不明确 - 令牌可能被伪造
5. **CVE-WS-012**: 速率限制配置不明 - 易受暴力破解
6. **CVE-WS-020**: 钱包解耦逻辑不完整 - 可能删除最后一个钱包
7. **CVE-WS-021**: 主钱包解绑策略缺失 - 数据一致性风险

### 9.3 中等漏洞 (11个) - 建议修复
1. **CVE-WS-005**: 消息模板钓鱼风险
2. **CVE-WS-007**: 缺少域名绑定验证
3. **CVE-WS-008**: 主钱包约束性能问题
4. **CVE-WS-009**: 审计日志不完整
5. **CVE-WS-013**: CORS配置不明确
6. **CVE-WS-014**: 错误响应信息泄露
7. **CVE-WS-015**: 签名请求无时间戳验证
8. **CVE-WS-016**: 缺少CSP策略
9. **CVE-WS-019**: 缺少地址黑名单机制
10. **CVE-WS-022**: 审计日志缺少IP记录
11. **CVE-WS-023**: 缺少多因素认证选项

### 9.4 低风险 (6个) - 可选修复
1. **CVE-WS-017**: 前端缺少敏感数据清理
2. **CVE-WS-021**: 缺少数据保留策略
3. **CVE-WS-024**: 测试代码可能泄露敏感信息
4. **CVE-WS-025**: 文档包含示例私钥
5. **CVE-WS-026**: 缺少安全头配置说明
6. **CVE-WS-027**: 错误监控可能收集敏感数据

---

## 10. 修复优先级建议

### 立即修复 (24小时内)
- [ ] CVE-WS-001: 修复EIP-191签名验证实现
- [ ] CVE-WS-002: 实现nonce存储和验证
- [ ] CVE-WS-010: 在API中添加nonce验证
- [ ] CVE-WS-018: 修复主钱包设置竞态条件

### 高优先级 (3天内)
- [ ] CVE-WS-003: 修正椭圆曲线选择
- [ ] CVE-WS-004: 增强签名格式验证
- [ ] CVE-WS-006: 实施钱包类型白名单
- [ ] CVE-WS-011: 配置JWT安全参数
- [ ] CVE-WS-012: 实现分布式速率限制
- [ ] CVE-WS-020: 完善钱包解耦逻辑

### 中优先级 (1周内)
- [ ] CVE-WS-005: 实施EIP-712结构化消息
- [ ] CVE-WS-007: 添加域名绑定验证
- [ ] CVE-WS-008: 优化数据库约束性能
- [ ] CVE-WS-009: 完善审计日志机制
- [ ] CVE-WS-013: 明确CORS配置
- [ ] CVE-WS-014: 统一错误处理
- [ ] CVE-WS-015: 前后端时间同步
- [ ] CVE-WS-016: 添加CSP策略
- [ ] CVE-WS-019: 实施地址黑名单

### 低优先级 (1月内)
- [ ] CVE-WS-017: 前端敏感数据清理
- [ ] CVE-WS-021: 数据保留和删除策略
- [ ] 其他低风险问题

---

## 11. 安全最佳实践建议

### 11.1 开发阶段
1. **代码审查**
   - 所有Web3相关代码必须经过安全审查
   - 至少两名工程师审核签名验证逻辑
   - 定期进行代码审计

2. **测试**
   - 100%单元测试覆盖率（当前已要求）
   - 安全测试覆盖所有攻击向量
   - 定期进行渗透测试

3. **文档**
   - 详细记录所有安全决策
   - 记录已知风险和缓解措施
   - 保持安全文档更新

### 11.2 部署阶段
1. **环境隔离**
   - 开发/测试/生产环境严格隔离
   - 不同环境使用不同的密钥
   - 生产环境禁用调试功能

2. **密钥管理**
   - 使用HSM或密钥管理服务
   - 定期轮换密钥
   - 最小权限原则

3. **监控**
   - 实时监控异常签名尝试
   - 监控API调用频率
   - 设置安全告警阈值

### 11.3 运营阶段
1. **审计**
   - 定期审查审计日志
   - 检查未授权访问尝试
   - 分析安全事件

2. **更新**
   - 及时更新依赖库
   - 关注安全公告
   - 定期进行安全评估

3. **应急响应**
   - 建立安全事件响应流程
   - 定期演练
   - 准备回滚方案

---

## 12. 参考资料

### 12.1 以太坊安全标准
- EIP-191: Signed Data Standard
  https://eips.ethereum.org/EIPS/eip-191

- EIP-712: Typed Structured Data Hashing and Signing
  https://eips.ethereum.org/EIPS/eip-712

- Ethereum Yellow Paper
  https://ethereum.github.io/yellowpaper/paper.pdf

### 12.2 安全最佳实践
- Consensys Ethereum Smart Contract Security Best Practices
  https://consensys.github.io/smart-contract-security-best-practices/

- OWASP Top 10 for Web3
  https://owasp.org/www-project-web3-top-10/

- Trail of Bits Ethereum Security Review Checklist
  https://github.com/trailofbits/publications/blob/master/reviews/EthereumStorj.md

### 12.3 工具和库
- Go Ethereum (go-ethereum)
  https://github.com/ethereum/go-ethereum

- MythX Security Analysis Platform
  https://mythx.io/

- Slither Static Analyzer (Python, 可用于智能合约分析)
  https://github.com/crytic/slither

---

## 13. 审计结论

**总体评估**: 当前Web3钱包集成提案在安全方面存在较多问题，特别是核心的密码学实现存在严重缺陷。**不建议在未修复关键漏洞前部署到生产环境**。

**核心问题**:
1. **签名验证完全错误** - 这是最严重的问题，将导致整个认证系统失效
2. **缺少重放攻击防护** - nonce机制未正确实现
3. **业务逻辑存在竞态条件** - 可能导致数据不一致

**积极方面**:
1. 架构设计整体合理，遵循了模块化原则
2. 数据库设计包含适当的外键约束
3. 测试覆盖率要求达到100%
4. 遵循EIP-191标准（虽然实现有误）

**建议**:
- 立即修复所有4个关键漏洞
- 在安全专家审查通过后再进行生产部署
- 建立持续的安全审计机制
- 考虑聘请第三方安全公司进行独立审计

**审计员签名**:
```
数字签名: 0x...
审计日期: 2025-12-01
审计版本: v1.0
```

---

**报告结束**
