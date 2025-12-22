package config

import (
	"os"
	"testing"
	"time"
)

func TestFeatureFlagManager_IsEnabled(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 测试默认启用的标志
	if !manager.IsEnabled(NewsAutoFetchEnabled) {
		t.Error("期望NewsAutoFetchEnabled为启用")
	}

	// 测试默认禁用的标志
	if manager.IsEnabled(BetaModeEnabled) {
		t.Error("期望BetaModeEnabled为禁用")
	}
}

func TestFeatureFlagManager_IsEnabledForUser_FullPercentage(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 设置为100%启用
	manager.SetPercentage(NewsAutoFetchEnabled, 100)

	// 应该对所有用户启用
	if !manager.IsEnabledForUser(NewsAutoFetchEnabled, "user-1") {
		t.Error("期望user-1启用该功能")
	}
	if !manager.IsEnabledForUser(NewsAutoFetchEnabled, "user-2") {
		t.Error("期望user-2启用该功能")
	}
}

func TestFeatureFlagManager_IsEnabledForUser_ZeroPercentage(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 设置为0%启用
	manager.SetPercentage(BetaModeEnabled, 0)
	manager.SetEnabled(BetaModeEnabled, true) // 启用标志本身

	// 不应该对任何用户启用
	if manager.IsEnabledForUser(BetaModeEnabled, "user-1") {
		t.Error("期望user-1禁用该功能")
	}
}

func TestFeatureFlagManager_IsEnabledForUser_PartialPercentage(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 设置为50%启用
	manager.SetPercentage(BetaModeEnabled, 50)
	manager.SetEnabled(BetaModeEnabled, true)

	// 某些用户应该启用，某些应该禁用
	// 结果取决于userID的哈希值
	results := make(map[string]bool)
	for i := 1; i <= 10; i++ {
		userID := "user-" + string(rune(48+i)) // user-1 to user-:
		results[userID] = manager.IsEnabledForUser(BetaModeEnabled, userID)
	}

	// 验证至少有一些用户启用，一些禁用（概率性的）
	enabledCount := 0
	for _, enabled := range results {
		if enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 || enabledCount == 10 {
		t.Error("期望50%的用户启用该功能")
	}
}

func TestFeatureFlagManager_SetEnabled(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 初始状态
	flag, _ := manager.GetFlag(NewsAutoFetchEnabled)
	if !flag.Enabled {
		t.Error("初始状态应该为启用")
	}

	// 禁用标志
	err := manager.SetEnabled(NewsAutoFetchEnabled, false)
	if err != nil {
		t.Errorf("设置失败: %v", err)
	}

	if manager.IsEnabled(NewsAutoFetchEnabled) {
		t.Error("期望标志现在禁用")
	}
}

func TestFeatureFlagManager_SetPercentage(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 设置有效百分比
	err := manager.SetPercentage(BetaModeEnabled, 75)
	if err != nil {
		t.Errorf("设置失败: %v", err)
	}

	flag, _ := manager.GetFlag(BetaModeEnabled)
	if flag.Percentage != 75 {
		t.Errorf("期望百分比为75，得到%d", flag.Percentage)
	}

	// 设置无效百分比
	err = manager.SetPercentage(BetaModeEnabled, 150)
	if err == nil {
		t.Error("应该拒绝无效的百分比")
	}
}

func TestFeatureFlagManager_SetMetadata(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 设置元数据
	err := manager.SetMetadata(NewsAutoFetchEnabled, "max_retries", 3)
	if err != nil {
		t.Errorf("设置元数据失败: %v", err)
	}

	flag, _ := manager.GetFlag(NewsAutoFetchEnabled)
	if flag.Metadata["max_retries"] != 3 {
		t.Error("元数据设置失败")
	}
}

func TestFeatureFlagManager_GetFlag(t *testing.T) {
	manager := NewFeatureFlagManager()

	flag, err := manager.GetFlag(NewsAutoFetchEnabled)
	if err != nil {
		t.Errorf("获取标志失败: %v", err)
	}

	if flag.Name != NewsAutoFetchEnabled {
		t.Error("标志名称不匹配")
	}

	// 测试不存在的标志
	_, err = manager.GetFlag("non.existent.flag")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestFeatureFlagManager_ListAllFlags(t *testing.T) {
	manager := NewFeatureFlagManager()

	flags := manager.ListAllFlags()
	if len(flags) == 0 {
		t.Error("期望至少有一些标志")
	}

	// 验证包含预期的标志
	hasNewsFlag := false
	for _, flag := range flags {
		if flag.Name == NewsAutoFetchEnabled {
			hasNewsFlag = true
			break
		}
	}

	if !hasNewsFlag {
		t.Error("期望包含NewsAutoFetchEnabled标志")
	}
}

func TestFeatureFlagManager_SaveAndLoadFromFile(t *testing.T) {
	manager := NewFeatureFlagManager()

	// 修改一些标志
	manager.SetEnabled(BetaModeEnabled, true)
	manager.SetPercentage(BetaModeEnabled, 25)
	manager.SetMetadata(BetaModeEnabled, "release_date", "2025-12-21")

	// 创建临时文件
	tmpFile := "test_flags.json"
	defer os.Remove(tmpFile)

	// 保存到文件
	err := manager.SaveToFile(tmpFile)
	if err != nil {
		t.Errorf("保存文件失败: %v", err)
	}

	// 创建新的管理器并加载
	manager2 := NewFeatureFlagManager()
	err = manager2.LoadFromFile(tmpFile)
	if err != nil {
		t.Errorf("加载文件失败: %v", err)
	}

	// 验证加载的数据
	flag, _ := manager2.GetFlag(BetaModeEnabled)
	if !flag.Enabled {
		t.Error("期望BetaModeEnabled为启用")
	}
	if flag.Percentage != 25 {
		t.Errorf("期望百分比为25，得到%d", flag.Percentage)
	}
	if flag.Metadata["release_date"] != "2025-12-21" {
		t.Error("元数据未正确加载")
	}
}

func TestFeatureFlagManager_UpdatedAt(t *testing.T) {
	manager := NewFeatureFlagManager()

	flag1, _ := manager.GetFlag(NewsAutoFetchEnabled)
	initialTime := flag1.UpdatedAt

	// 等待一点时间
	time.Sleep(10 * time.Millisecond)

	// 更新标志
	manager.SetEnabled(NewsAutoFetchEnabled, false)

	flag2, _ := manager.GetFlag(NewsAutoFetchEnabled)
	if !flag2.UpdatedAt.After(initialTime) {
		t.Error("UpdatedAt应该被更新")
	}
}

// 辅助函数
func TestHashUserID(t *testing.T) {
	// 测试哈希函数
	hash1 := hashUserID("user-1")
	hash2 := hashUserID("user-1")
	hash3 := hashUserID("user-2")

	// 相同的userID应该产生相同的哈希
	if hash1 != hash2 {
		t.Error("相同userID应该产生相同的哈希")
	}

	// 不同的userID可能产生不同的哈希（虽然哈希碰撞是可能的）
	// 但我们可以测试结果在0-99范围内
	if hash1 < 0 || hash1 >= 100 {
		t.Errorf("哈希值应该在0-99范围内，得到%d", hash1)
	}

	if hash3 < 0 || hash3 >= 100 {
		t.Errorf("哈希值应该在0-99范围内，得到%d", hash3)
	}
}
