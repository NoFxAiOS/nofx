package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"time"
)

// UserStore 用户存储
type UserStore struct {
	db *sql.DB
}

// User 用户
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	OTPSecret    string    `json:"-"`
	OTPVerified  bool      `json:"otp_verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

func (s *UserStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 触发器
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_users_updated_at
		AFTER UPDATE ON users
		BEGIN
			UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return err
	}

	return nil
}

// Create 创建用户
func (s *UserStore) Create(user *User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (id, email, password_hash, otp_secret, otp_verified)
		VALUES (?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified)
	return err
}

// GetByEmail 通过邮箱获取用户
func (s *UserStore) GetByEmail(email string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &user, nil
}

// GetByID 通过ID获取用户
func (s *UserStore) GetByID(userID string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &user, nil
}

// GetAllIDs 获取所有用户ID
func (s *UserStore) GetAllIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

// UpdateOTPVerified 更新OTP验证状态
func (s *UserStore) UpdateOTPVerified(userID string, verified bool) error {
	_, err := s.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// UpdatePassword 更新密码
func (s *UserStore) UpdatePassword(userID, passwordHash string) error {
	_, err := s.db.Exec(`
		UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, passwordHash, userID)
	return err
}

// EnsureAdmin 确保admin用户存在
func (s *UserStore) EnsureAdmin() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.Create(&User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "",
		OTPSecret:    "",
		OTPVerified:  true,
	})
}
