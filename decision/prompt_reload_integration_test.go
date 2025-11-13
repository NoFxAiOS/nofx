package decision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPromptReloadEndToEnd 端到端测试：验证从文件修改到决策引擎使用的完整流程
func TestPromptReloadEndToEnd(t *testing.T) {
	// 保存原始的 promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		// 恢复原始模板
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// 创建临时目录模拟 prompts/ 目录
	tempDir := t.TempDir()
	promptsDir = tempDir

	// 步骤1: 创建初始 prompt 文件
	initialContent := "# 初始交易策略\n你是一个保守的交易AI。"
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(initialContent), 0644); err != nil {
		t.Fatalf("创建初始文件失败: %v", err)
	}

	// 步骤2: 首次加载（模拟系统启动）
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("首次加载失败: %v", err)
	}

	// 步骤3: 验证初始内容
	template, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("获取初始模板失败: %v", err)
	}
	if template.Content != initialContent {
		t.Errorf("初始内容不匹配\n期望: %s\n实际: %s", initialContent, template.Content)
	}

	// 步骤4: 使用 buildSystemPrompt 验证模板被正确使用
	systemPrompt := buildSystemPrompt(10000.0, 10, 5, "test_strategy")
	if !strings.Contains(systemPrompt, initialContent) {
		t.Errorf("buildSystemPrompt 未包含模板内容\n生成的 prompt:\n%s", systemPrompt)
	}

	// 步骤5: 模拟用户修改文件（这是用户在硬盘上修改 prompt）
	updatedContent := "# 更新的交易策略\n你是一个激进的交易AI，追求高风险高收益。"
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(updatedContent), 0644); err != nil {
		t.Fatalf("更新文件失败: %v", err)
	}

	// 步骤6: 模拟交易员启动时调用 ReloadPromptTemplates()
	t.Log("模拟交易员启动，调用 ReloadPromptTemplates()...")
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}

	// 步骤7: 验证新内容已生效
	reloadedTemplate, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("获取重新加载的模板失败: %v", err)
	}
	if reloadedTemplate.Content != updatedContent {
		t.Errorf("重新加载后内容不匹配\n期望: %s\n实际: %s", updatedContent, reloadedTemplate.Content)
	}

	// 步骤8: 验证 buildSystemPrompt 使用了新内容
	newSystemPrompt := buildSystemPrompt(10000.0, 10, 5, "test_strategy")
	if !strings.Contains(newSystemPrompt, updatedContent) {
		t.Errorf("buildSystemPrompt 未包含更新后的模板内容\n生成的 prompt:\n%s", newSystemPrompt)
	}

	// 步骤9: 验证旧内容不再存在
	if strings.Contains(newSystemPrompt, "保守的交易AI") {
		t.Errorf("buildSystemPrompt 仍包含旧的模板内容")
	}

	t.Log("✅ 端到端测试通过：文件修改 -> 重新加载 -> 决策引擎使用新内容")
}

// TestPromptReloadWithCustomPrompt 测试自定义 prompt 与模板重新加载的交互
func TestPromptReloadWithCustomPrompt(t *testing.T) {
	// 保存原始的 promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// 创建临时目录
	tempDir := t.TempDir()
	promptsDir = tempDir

	// 创建基础模板
	baseContent := "基础策略：稳健交易"
	if err := os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 加载模板
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	// 测试1: 基础模板 + 自定义 prompt（不覆盖）
	customPrompt := "个性化规则：只交易 BTC"
	result := buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, false, "base")
	if !strings.Contains(result, baseContent) {
		t.Errorf("未包含基础模板内容")
	}
	if !strings.Contains(result, customPrompt) {
		t.Errorf("未包含自定义 prompt")
	}

	// 测试2: 覆盖基础 prompt
	result = buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, true, "base")
	if strings.Contains(result, baseContent) {
		t.Errorf("覆盖模式下仍包含基础模板内容")
	}
	if !strings.Contains(result, customPrompt) {
		t.Errorf("覆盖模式下未包含自定义 prompt")
	}

	// 测试3: 重新加载后效果
	updatedBase := "更新的基础策略：激进交易"
	if err := os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte(updatedBase), 0644); err != nil {
		t.Fatalf("更新文件失败: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}

	result = buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, false, "base")
	if !strings.Contains(result, updatedBase) {
		t.Errorf("重新加载后未包含更新的基础模板内容")
	}
	if strings.Contains(result, baseContent) {
		t.Errorf("重新加载后仍包含旧的基础模板内容")
	}
}

// TestPromptReloadFallback 测试模板不存在时的降级机制
func TestPromptReloadFallback(t *testing.T) {
	// 保存原始的 promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// 创建临时目录
	tempDir := t.TempDir()
	promptsDir = tempDir

	// 只创建 default 模板
	defaultContent := "默认策略"
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte(defaultContent), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	// 测试1: 请求不存在的模板，应该降级到 default
	result := buildSystemPrompt(10000.0, 10, 5, "nonexistent")
	if !strings.Contains(result, defaultContent) {
		t.Errorf("请求不存在的模板时，未降级到 default")
	}

	// 测试2: 空模板名，应该使用 default
	result = buildSystemPrompt(10000.0, 10, 5, "")
	if !strings.Contains(result, defaultContent) {
		t.Errorf("空模板名时，未使用 default")
	}
}

// TestConcurrentPromptReload 测试并发场景下的 prompt 重新加载
func TestConcurrentPromptReload(t *testing.T) {
	// 保存原始的 promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// 创建临时目录
	tempDir := t.TempDir()
	promptsDir = tempDir

	// 创建测试文件
	if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("测试内容"), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("初始加载失败: %v", err)
	}

	// 并发测试：同时读取和重新加载
	done := make(chan bool)

	// 启动多个读取 goroutine
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = GetPromptTemplate("test")
			}
			done <- true
		}()
	}

	// 启动多个重新加载 goroutine
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = ReloadPromptTemplates()
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 13; i++ {
		<-done
	}

	// 验证最终状态正确
	template, err := GetPromptTemplate("test")
	if err != nil {
		t.Errorf("并发测试后获取模板失败: %v", err)
	}
	if template.Content != "测试内容" {
		t.Errorf("并发测试后模板内容错误: %s", template.Content)
	}

	t.Log("✅ 并发测试通过：多个 goroutine 同时读取和重新加载模板，无数据竞争")
}
