package config

import (
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to setup DB
func setupInvitationTestDB(t *testing.T) *Database {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		t.Skip("Skipping integration test: TEST_DATABASE_URL not set")
	}

	// Connect using the existing NewDatabase which handles migration
	// Note: NewDatabase expects DATABASE_URL env var, but we pass empty string?
	// No, NewDatabase reads os.Getenv("DATABASE_URL").

	originalURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", testDBURL)
	defer os.Setenv("DATABASE_URL", originalURL)

	db, err := NewDatabase("")
	require.NoError(t, err)

	return db
}

func cleanupInvitationData(db *Database) {
	db.Exec("DELETE FROM credit_transactions")
	db.Exec("DELETE FROM user_credits")
	db.Exec("DELETE FROM users WHERE email LIKE 'invite_test_%'")
}

// Unit Test: Code Generation
func TestGenerateInviteCode(t *testing.T) {
	code := GenerateInviteCode()
	assert.Len(t, code, 8)
	assert.Regexp(t, "^[A-Z0-9]+$", code)
}

// Integration Test: Full Invitation Flow
func TestInvitationIntegration(t *testing.T) {
	db := setupInvitationTestDB(t)
	defer db.Close()
	defer cleanupInvitationData(db)

	// 1. Create Inviter
	inviterEmail := fmt.Sprintf("invite_test_inviter_%d@example.com", time.Now().UnixNano())
	inviter := &User{
		ID:       "inviter_" + GenerateUUID(),
		Email:    inviterEmail,
		IsActive: true,
	}
	// CreateInviter using CreateUserWithInvitation to ensure they get a code
	err := db.CreateUserWithInvitation(inviter)
	require.NoError(t, err)

	// Refresh to get code
	savedInviter, err := db.GetUserByID(inviter.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, savedInviter.InviteCode)
	inviterCode := savedInviter.InviteCode

	// 2. Create Invitee using the code
	inviteeEmail := fmt.Sprintf("invite_test_invitee_%d@example.com", time.Now().UnixNano())
	invitee := &User{
		ID:              "invitee_" + GenerateUUID(),
		Email:           inviteeEmail,
		InvitedByUserID: inviter.ID,
		InvitationLevel: savedInviter.InvitationLevel + 1,
		IsActive:        true,
	}

	// Note: In real app, API looks up user by code to get ID. Here we simulate that look up passed.
	// Verify GetUserByInviteCode works
	lookupInviter, err := db.GetUserByInviteCode(inviterCode)
	require.NoError(t, err)
	assert.Equal(t, inviter.ID, lookupInviter.ID)

	// Create invitee
	err = db.CreateUserWithInvitation(invitee)
	require.NoError(t, err)

	// 3. Verify Relationships
	savedInvitee, err := db.GetUserByID(invitee.ID)
	require.NoError(t, err)
	assert.Equal(t, inviter.ID, savedInvitee.InvitedByUserID)
	assert.Equal(t, savedInviter.InvitationLevel+1, savedInvitee.InvitationLevel)

	// 4. Verify Credits Awarded to Inviter
	credits, err := db.GetUserCredits(inviter.ID)
	require.NoError(t, err)
	assert.NotNil(t, credits)
	assert.Equal(t, 10, credits.AvailableCredits)

	// Verify Transaction Log
	txs, total, err := db.GetUserTransactions(inviter.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "referral_reward", txs[0].Category)
	assert.Equal(t, 10, txs[0].Amount)
}

// Test Duplicate Invite Code Retry (Mocking hard here, so we skip or just test uniqueness)
func TestUniqueCodes(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code := GenerateInviteCode()
		if codes[code] {
			t.Fatalf("Collision detected after %d iterations: %s", i, code)
		}
		codes[code] = true
	}
}
