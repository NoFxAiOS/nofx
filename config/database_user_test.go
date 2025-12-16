package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetAllUsers 测试获取所有用户
func TestGetAllUsers(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建几个测试用户
	now := time.Now()
	users := []*User{
		{ID: "test_user_10", Email: "test10@example.com", PasswordHash: "hash1", CreatedAt: now, UpdatedAt: now},
		{ID: "test_user_11", Email: "test11@example.com", PasswordHash: "hash2", CreatedAt: now, UpdatedAt: now},
		{ID: "test_user_12", Email: "test12@example.com", PasswordHash: "hash3", CreatedAt: now, UpdatedAt: now},
	}

	for _, user := range users {
		err := tdb.db.CreateUser(user)
		require.NoError(t, err)
	}

	// 获取所有用户
	allUsers, err := tdb.db.GetAllUsers()
	assert.NoError(t, err, "Should be able to get all users")
	assert.NotNil(t, allUsers, "User list should not be nil")
	assert.GreaterOrEqual(t, len(allUsers), 3, "Should have at least 3 test users")

	// 验证测试用户在列表中
	userMap := make(map[string]bool)
	for _, userID := range allUsers {
		userMap[userID] = true
	}
	assert.True(t, userMap["test_user_10"], "test_user_10 should be in list")
	assert.True(t, userMap["test_user_11"], "test_user_11 should be in list")
	assert.True(t, userMap["test_user_12"], "test_user_12 should be in list")
}

// TestGetUsers 测试分页获取用户
func TestGetUsers(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	for i := 20; i < 25; i++ {
		user := &User{
			ID:           "test_user_" + string(rune(i)),
			Email:        "test" + string(rune(i)) + "@example.com",
			PasswordHash: "hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err := tdb.db.CreateUser(user)
		require.NoError(t, err)
	}

	// 测试分页
	users, total, err := tdb.db.GetUsers(1, 10, "", "created_at", "desc")
	assert.NoError(t, err, "Should be able to get users")
	assert.NotNil(t, users, "Users should not be nil")
	assert.Greater(t, total, 0, "Total should be greater than 0")
	assert.LessOrEqual(t, len(users), 10, "Should return at most 10 users")
}

// TestGetUsersWithSearch 测试搜索用户
func TestGetUsersWithSearch(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_search",
		Email:        "searchable@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 搜索用户
	users, total, err := tdb.db.GetUsers(1, 10, "searchable", "created_at", "desc")
	assert.NoError(t, err, "Should be able to search users")
	assert.Greater(t, len(users), 0, "Should find at least one user")
	assert.Greater(t, total, 0, "Total should be greater than 0")

	// 验证搜索结果
	found := false
	for _, u := range users {
		if u.Email == "searchable@example.com" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find the searchable user")
}

// TestUpdateUserPassword 测试更新用户密码
func TestUpdateUserPassword(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_pwd",
		Email:        "testpwd@example.com",
		PasswordHash: "old_hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新密码
	err = tdb.db.UpdateUserPassword("test_user_pwd", "new_hash")
	assert.NoError(t, err, "Should be able to update password")

	// 验证密码已更新
	updatedUser, err := tdb.db.GetUserByID("test_user_pwd")
	assert.NoError(t, err)
	assert.Equal(t, "new_hash", updatedUser.PasswordHash, "Password should be updated")
}

// TestUpdateUserLockoutStatus 测试更新用户锁定状态
func TestUpdateUserLockoutStatus(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_lock",
		Email:        "testlock@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新锁定状态
	lockUntil := time.Now().Add(1 * time.Hour)
	err = tdb.db.UpdateUserLockoutStatus("test_user_lock", 3, &lockUntil)
	assert.NoError(t, err, "Should be able to update lockout status")

	// 验证锁定状态已更新
	updatedUser, err := tdb.db.GetUserByID("test_user_lock")
	assert.NoError(t, err)
	assert.Equal(t, 3, updatedUser.FailedAttempts, "Failed attempts should be 3")
	assert.NotNil(t, updatedUser.LockedUntil, "LockedUntil should not be nil")
}

// TestResetUserFailedAttempts 测试重置失败尝试次数
func TestResetUserFailedAttempts(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	lockUntil := now.Add(1 * time.Hour)
	user := &User{
		ID:             "test_user_reset",
		Email:          "testreset@example.com",
		PasswordHash:   "hash",
		FailedAttempts: 5,
		LockedUntil:    &lockUntil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 重置失败尝试
	err = tdb.db.ResetUserFailedAttempts("test_user_reset")
	assert.NoError(t, err, "Should be able to reset failed attempts")

	// 验证已重置
	updatedUser, err := tdb.db.GetUserByID("test_user_reset")
	assert.NoError(t, err)
	assert.Equal(t, 0, updatedUser.FailedAttempts, "Failed attempts should be 0")
	assert.Nil(t, updatedUser.LockedUntil, "LockedUntil should be nil")
}

// TestUpdateUserOTPVerified 测试更新OTP验证状态
func TestUpdateUserOTPVerified(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_otp",
		Email:        "testotp@example.com",
		PasswordHash: "hash",
		OTPVerified:  false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新OTP验证状态
	err = tdb.db.UpdateUserOTPVerified("test_user_otp", true)
	assert.NoError(t, err, "Should be able to update OTP verified status")

	// 验证已更新
	updatedUser, err := tdb.db.GetUserByID("test_user_otp")
	assert.NoError(t, err)
	assert.True(t, updatedUser.OTPVerified, "OTP should be verified")
}

// TestGetUserCount 测试获取用户总数
func TestGetUserCount(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	for i := 30; i < 35; i++ {
		user := &User{
			ID:           "test_user_count_" + string(rune(i)),
			Email:        "count" + string(rune(i)) + "@example.com",
			PasswordHash: "hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err := tdb.db.CreateUser(user)
		require.NoError(t, err)
	}

	// 获取总数
	count, err := tdb.db.GetUserCount("")
	assert.NoError(t, err, "Should be able to get user count")
	assert.Greater(t, count, 0, "Count should be greater than 0")

	// 带搜索的总数
	countWithSearch, err := tdb.db.GetUserCount("count")
	assert.NoError(t, err, "Should be able to get user count with search")
	assert.GreaterOrEqual(t, countWithSearch, 5, "Should find at least 5 users with 'count' in email")
}
