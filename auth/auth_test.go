package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSecurityJWTValidation(t *testing.T) {
	previous := JWTSecret
	SetJWTSecret("isolated-jwt-regression-secret")
	t.Cleanup(func() { JWTSecret = previous })
	token, err := GenerateJWT("owner", "owner@example.test", 7)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateJWT(token)
	if err != nil || claims.AuthVersion != 7 || claims.ID == "" {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	for _, kind := range []string{"wrong-algorithm", "wrong-issuer", "missing-expiry", "expired", "missing-user", "missing-id", "negative-version"} {
		t.Run(kind, func(t *testing.T) {
			c := *claims
			var method jwt.SigningMethod = jwt.SigningMethodHS256
			switch kind {
			case "wrong-algorithm":
				method = jwt.SigningMethodHS512
			case "wrong-issuer":
				c.Issuer = "other"
			case "missing-expiry":
				c.ExpiresAt = nil
			case "expired":
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			case "missing-user":
				c.UserID = ""
			case "missing-id":
				c.ID = ""
			case "negative-version":
				c.AuthVersion = -1
			}
			raw, err := jwt.NewWithClaims(method, c).SignedString(JWTSecret)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateJWT(raw); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	// Pre-upgrade logout records were memory-only. All such tokens must require
	// a fresh login so previously logged-out sessions cannot revive on upgrade.
	legacy := jwt.MapClaims{"user_id": "owner", "email": "owner@example.test", "iss": "nofxAI", "exp": time.Now().Add(time.Hour).Unix()}
	raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString(JWTSecret)
	if _, err := ValidateJWT(raw); err == nil {
		t.Fatal("pre-hardening session was revived")
	}
}
