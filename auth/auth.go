package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret is the JWT secret key, will be dynamically set from config
var JWTSecret []byte

// SetJWTSecret sets the JWT secret key
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// Claims represents JWT claims
type Claims struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	AuthVersion int64  `json:"auth_version"`
	jwt.RegisteredClaims
}

// HashPassword hashes the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies the password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT generates JWT token
func GenerateJWT(userID, email string, authVersion int64) (string, error) {
	if len(JWTSecret) == 0 || userID == "" {
		return "", fmt.Errorf("JWT secret and user ID are required")
	}
	claims := Claims{
		UserID:      userID,
		Email:       email,
		AuthVersion: authVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Expires in 24 hours
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateJWT validates JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	if len(JWTSecret) == 0 {
		return nil, fmt.Errorf("JWT secret is not configured")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("nofxAI"), jwt.WithExpirationRequired())

	if err != nil {
		return nil, err
	}

	// Pre-hardening tokens had no unique ID and only in-memory revocation.
	// Do not resurrect those logged-out tokens when upgrading/restarting.
	if claims, ok := token.Claims.(*Claims); ok && token.Valid && claims.ID != "" && claims.UserID != "" && claims.AuthVersion >= 0 {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
