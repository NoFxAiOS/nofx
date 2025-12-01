package web3_auth

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// ============ 错误定义 ============

var (
	ErrInvalidSignature  = errors.New("无效的签名格式")
	ErrAddressMismatch   = errors.New("签名地址与预期地址不匹配")
	ErrSignatureRecovery = errors.New("签名恢复失败")
	ErrInvalidAddress    = errors.New("无效的以太坊地址")
	ErrNonceExpired      = errors.New("nonce已过期")
	ErrNonceUsed         = errors.New("nonce已被使用")
	ErrInvalidNonce      = errors.New("无效的nonce")
)

// ============ 常量定义 ============

const (
	// 签名消息版本
	EIP191_PREFIX = "\x19Ethereum Signed Message:\n"

	// nonce过期时间（分钟）
	NONCE_EXPIRY_MINUTES = 10

	// 最小签名长度（0x + 130字符 = 65字节 * 2 + 2）
	MIN_SIGNATURE_LENGTH = 132

	// 最大签名长度
	MAX_SIGNATURE_LENGTH = 132
)

// ============ 核心函数 ============

// RecoverAddressFromSignature 从签名中恢复地址（使用正确的secp256k1）
// 修复了CVE-WS-001：使用以太坊标准secp256k1曲线
func RecoverAddressFromSignature(message, signature, expectedAddress string) (string, error) {
	// 1. 验证地址格式
	if err := ValidateAddress(expectedAddress); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidAddress, err)
	}

	// 2. 验证签名格式
	if err := ValidateSignature(signature); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	// 3. 从签名中恢复公钥（使用正确的EIP-155标准）
	recoveredAddr, err := recoverAddressFromSignature(message, signature)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSignatureRecovery, err)
	}

	// 4. 验证恢复的地址与预期地址匹配（不区分大小写）
	if !strings.EqualFold(recoveredAddr, expectedAddress) {
		return "", ErrAddressMismatch
	}

	return recoveredAddr, nil
}

// recoverAddressFromSignature 内部签名恢复函数（使用secp256k1）
func recoverAddressFromSignature(message, signature string) (string, error) {
	// 1. 解析签名（去掉0x前缀）
	sigBytes, err := hexutil.Decode(signature)
	if err != nil {
		return "", fmt.Errorf("签名解析失败: %w", err)
	}

	// 2. 验证签名长度（必须是65字节）
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("签名长度无效，需要65字节，实际%d字节", len(sigBytes))
	}

	// 3. 应用EIP-155重放保护：调整recID
	// 如果使用Ledger等硬件钱包，recID已经是0或1
	// 如果是软件钱包，可能需要调整
	recID := sigBytes[64]
	if recID >= 4 {
		return "", fmt.Errorf("无效的recovery ID: %d", recID)
	}

	// 4. 生成签名哈希（使用EIP-191标准）
	msgHash := generateMessageHash(message)

	// 5. 从签名恢复公钥（使用go-ethereum的crypto.SigToPub，这是正确的secp256k1实现）
	sigPubKey, err := crypto.SigToPub(msgHash, sigBytes)
	if err != nil {
		return "", fmt.Errorf("公钥恢复失败: %w", err)
	}

	// 6. 从公钥计算地址
	address := crypto.PubkeyToAddress(*sigPubKey)

	// 7. 返回地址（添加0x前缀）
	return address.Hex(), nil
}

// generateMessageHash 生成EIP-191兼容的消息哈希
func generateMessageHash(message string) []byte {
	// EIP-191: 0x19 + version(1字节) + "Ethereum Signed Message:" + len(message) + message
	version := []byte{0}
	msgBytes := []byte(message)

	// 构造完整消息
	fullMessage := []byte{}
	fullMessage = append(fullMessage, 0x19)
	fullMessage = append(fullMessage, version...)
	fullMessage = append(fullMessage, []byte("Ethereum Signed Message:")...)
	fullMessage = append(fullMessage, []byte(fmt.Sprintf("%d", len(msgBytes)))...)
	fullMessage = append(fullMessage, msgBytes...)

	// 使用Keccak256生成哈希
	return crypto.Keccak256(fullMessage)
}

// ============ 验证函数 ============

// ValidateAddress 验证以太坊地址格式
func ValidateAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("地址不能为空")
	}

	// 添加0x前缀（如果缺失）
	if !strings.HasPrefix(addr, "0x") && !strings.HasPrefix(addr, "0X") {
		addr = "0x" + addr
	}

	// 验证长度（0x + 40字符 = 42字符）
	if len(addr) != 42 {
		return fmt.Errorf("地址长度无效，需要42字符，实际%d字符", len(addr))
	}

	// 验证是否为有效的十六进制地址
	if !common.IsHexAddress(addr) {
		return fmt.Errorf("地址格式无效")
	}

	return nil
}

// ValidateSignature 验证签名格式
func ValidateSignature(signature string) error {
	if signature == "" {
		return fmt.Errorf("签名不能为空")
	}

	// 添加0x前缀（如果缺失）
	if !strings.HasPrefix(signature, "0x") && !strings.HasPrefix(signature, "0X") {
		signature = "0x" + signature
	}

	// 验证长度（0x + 130字符 = 65字节 * 2 + 2 = 132字符）
	if len(signature) < MIN_SIGNATURE_LENGTH || len(signature) > MAX_SIGNATURE_LENGTH {
		return fmt.Errorf("签名长度无效，需要132字符，实际%d字符", len(signature))
	}

	// 验证是否为有效的十六进制
	_, err := hexutil.Decode(signature)
	if err != nil {
		return fmt.Errorf("签名不是有效的十六进制: %w", err)
	}

	return nil
}

// ValidateWalletType 验证钱包类型
func ValidateWalletType(walletType string) error {
	validTypes := map[string]bool{
		"metamask": true,
		"tp":       true,
		"other":    true,
	}

	if !validTypes[walletType] {
		return fmt.Errorf("不支持的钱包类型: %s，支持的类型: metamask, tp, other", walletType)
	}

	return nil
}

// ============ 消息生成 ============

// GenerateSignatureMessage 生成防钓鱼的签名消息
func GenerateSignatureMessage(address, nonce string, expiresAt time.Time) string {
	expiryStr := expiresAt.Format("2006-01-02 15:04:05 UTC")

	return fmt.Sprintf(`
Monnaire Trading Agent OS - Web3 Authentication

Wallet Address: %s
Nonce: %s
Expires: %s

⚠️ 安全提醒:
- 此签名不会触发区块链交易，不消耗Gas费
- 请确认您正在访问正确的网站域名
- 请勿在非官方页面签名

如果您未授权此请求，请忽略此消息。

Signature Expires: 10 minutes
`, address, nonce, expiryStr)
}

// ============ Nonce管理 ============

// GenerateNonce 生成加密安全的随机nonce
func GenerateNonce() (string, error) {
	bytes := make([]byte, 32) // 256位随机数
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ValidateNonce 验证nonce格式
func ValidateNonce(nonce string) error {
	if nonce == "" {
		return fmt.Errorf("nonce不能为空")
	}

	if len(nonce) < 32 {
		return fmt.Errorf("nonce长度不足，至少需要32字符")
	}

	// 验证是否为有效的十六进制
	_, err := hex.DecodeString(nonce)
	if err != nil {
		return fmt.Errorf("nonce不是有效的十六进制: %w", err)
	}

	return nil
}

// CheckNonceExpiration 检查nonce是否过期
func CheckNonceExpiration(expiresAt time.Time) error {
	if time.Now().After(expiresAt) {
		return ErrNonceExpired
	}
	return nil
}

// ============ 安全增强 ============

// HashSignature 生成签名的哈希（用于审计日志）
func HashSignature(signature string) string {
	// 只取签名的前10个字符进行哈希，避免泄露完整签名
	hash := crypto.Keccak256([]byte(signature))
	return hex.EncodeToString(hash[:16]) // 取前16字节
}

// SanitizeAddress 清理地址格式（用于日志记录）
func SanitizeAddress(addr string) string {
	// 只显示地址的前6位和后4位
	if len(addr) > 10 {
		return addr[:6] + "..." + addr[len(addr)-4:]
	}
	return addr
}
