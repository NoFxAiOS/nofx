package auth

import (
	"testing"
	"time"
)

func TestTokenExpiration(t *testing.T) {
	// 设置 JWT secret
	SetJWTSecret("test_secret")

	// 设置 token 只存活 1 分钟
	SetTokenExpiration("1")

	token, err := GenerateJWT("test", "test@nofx.ai")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 验证是成功的
	_, err = ValidateJWT(token)
	if err != nil {
		t.Fatalf("Token should be valid immediately after creation, got error: %v", err)
	}

	// 等待 token 过期（>61 秒）
	time.Sleep(61 * time.Second)

	// 再验证应失败
	_, err = ValidateJWT(token)
	if err == nil {
		t.Fatalf("Expected token to be expired, but ValidateJWT returned no error")
	}
}
