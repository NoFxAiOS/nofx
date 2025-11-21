package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret JWT密钥，将从配置中动态设置
var JWTSecret []byte

// OTPIssuer OTP发行者名称
const OTPIssuer = "nofxAI"

// 错误定义
var (
	ErrPasswordTooShort = errors.New("密码强度不足（至少8位）")
	ErrInvalidEmail     = errors.New("邮箱格式不正确")
	ErrInvalidOTP       = errors.New("验证码错误")
	ErrTokenExpired     = errors.New("token已过期")
	ErrInvalidToken     = errors.New("token无效")
)

// SetJWTSecret 设置JWT密钥
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// Claims JWT声明
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      OTPIssuer,
		AccountName: uuid.New().String(),
	})
	if err != nil {
		return "", err
	}

	return key.Secret(), nil
}

// VerifyOTP 验证OTP码
func VerifyOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateJWT 生成JWT token
func GenerateJWT(userID, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24小时过期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateJWT 验证JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("无效的token")
}

// GetOTPQRCodeURL 获取OTP二维码URL
func GetOTPQRCodeURL(secret, email string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", OTPIssuer, email, secret, OTPIssuer)
}

// ValidateEmail 验证邮箱格式
func ValidateEmail(email string) error {
	if email == "" {
		return ErrInvalidEmail
	}

	// 简单的邮箱验证
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return ErrInvalidEmail
	}

	if len(email) < 5 || len(email) > 254 {
		return ErrInvalidEmail
	}

	return nil
}

// GeneratePasswordResetToken 生成密码重置令牌
func GeneratePasswordResetToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("生成令牌失败: %w", err)
	}
	return hex.EncodeToString(tokenBytes), nil
}

// HashPasswordResetToken 哈希密码重置令牌
func HashPasswordResetToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// IsAccountLocked 检查账户是否被锁定
func IsAccountLocked(lockedUntil *time.Time) bool {
	if lockedUntil == nil {
		return false
	}
	return time.Now().Before(*lockedUntil)
}

// GetUnlockTime 获取解锁时间
func GetUnlockTime(lockedUntil *time.Time) *time.Time {
	if lockedUntil == nil {
		return nil
	}
	return lockedUntil
}

// ExtractIPFromRequest 从请求头中提取IP地址（简化版）
func ExtractIPFromRequest(headers map[string]string) string {
	// 检查X-Forwarded-For头（如果存在）
	if forwardedFor, ok := headers["X-Forwarded-For"]; ok && forwardedFor != "" {
		// X-Forwarded-For可能包含多个IP，取第一个
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// 检查X-Real-IP头
	if realIP, ok := headers["X-Real-IP"]; ok && realIP != "" {
		return realIP
	}

	// 默认返回空字符串，实际应用中应该从TCP连接中获取
	return ""
}

// GenerateUserID 生成用户ID
func GenerateUserID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}
