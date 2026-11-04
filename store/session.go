package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Only digests of signed session identities are stored, never bearer tokens.
// Keying by raw token text is unsafe: base64url signatures can have equivalent
// encodings. Revocation must cover every encoding of the same signed session.
type revokedSession struct {
	SessionHash string    `gorm:"primaryKey;size:64"`
	ExpiresAt   time.Time `gorm:"not null;index"`
}

func sessionHash(userID, sessionID string) string {
	sum := sha256.Sum256([]byte(userID + "\x00" + sessionID))
	return hex.EncodeToString(sum[:])
}

func (s *UserStore) RevokeSession(userID, sessionID string, expiresAt time.Time) error {
	if userID == "" || sessionID == "" {
		return errors.New("session identity is required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", time.Now().UTC()).Delete(&revokedSession{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&revokedSession{
			SessionHash: sessionHash(userID, sessionID), ExpiresAt: expiresAt.UTC(),
		}).Error
	})
}

func (s *UserStore) IsSessionRevoked(userID, sessionID string) (bool, error) {
	if userID == "" || sessionID == "" {
		return false, errors.New("session identity is required")
	}
	var count int64
	err := s.db.Model(&revokedSession{}).Where("session_hash = ? AND expires_at > ?", sessionHash(userID, sessionID), time.Now().UTC()).Count(&count).Error
	return count != 0, err
}
