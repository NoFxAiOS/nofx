package web3_auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT安全配置
const (
	// Token过期时间
	JWTExpiryHours = 24

	// 刷新Token过期时间
	RefreshTokenExpiryHours = 168 // 7天

	// 允许的时钟偏差（防止时钟漂移）
	ClockSkewLeeway = 5 * time.Second
)

// Claims Web3钱包认证的JWT声明
type Claims struct {
	UserID    string   `json:"user_id"`
	WalletAddr string  `json:"wallet_addr"`
	TokenType string   `json:"token_type"` // "access" or "refresh"
	Wallets   []string `json:"wallets"`    // 用户绑定的所有钱包地址
	jwt.RegisteredClaims
}

// 生成JWT令牌（安全配置）
func GenerateWeb3JWT(userID, walletAddr string, walletList []string, isRefresh bool) (string, error) {
	var expiryTime time.Time
	var tokenType string

	if isRefresh {
		expiryTime = time.Now().Add(RefreshTokenExpiryHours * time.Hour)
		tokenType = "refresh"
	} else {
		expiryTime = time.Now().add(JWTExpiryHours * time.Hour)
		tokenType = "access"
	}

	claims := Claims{
		UserID:    userID,
		WalletAddr: walletAddr,
		TokenType: tokenType,
		Wallets:   walletList,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime.Add(-ClockSkewLeeway)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-ClockSkewLeeway)),
			Issuer:    "Monnaire Trading Agent OS",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWTSecret))
}

// 验证JWT令牌（安全配置）
func ValidateWeb3JWT(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("token不能为空")
	}

	// 解析token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("意外的签名方法: " + token.Header["alg"].(string))
		}

		// 验证算法
		if token.Header["alg"] != "HS256" {
			return nil, errors.New("不允许的算法: " + token.Header["alg"].(string))
		}

		return []byte(JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	// 验证token有效性
	if !token.Valid {
		return nil, errors.New("token无效")
	}

	// 强制验证过期时间
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("无法获取claims")
	}

	// 验证过期时间
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time.Add(ClockSkewLeeway)) {
		return nil, errors.New("token已过期")
	}

	// 验证发行者
	if claims.Issuer != "Monnaire Trading Agent OS" {
		return nil, errors.New("无效的发行者")
	}

	return claims, nil
}

// 从请求头中提取Token
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("授权头为空")
	}

	// 支持 "Bearer <token>" 格式
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:], nil
	}

	// 支持 "Basic <credentials>" 格式（不推荐用于Web3）
	if len(authHeader) > 6 && authHeader[:6] == "Basic " {
		return "", errors.New("不支持Basic认证，请使用Bearer token")
	}

	return "", errors.New("无效的授权格式")
}
