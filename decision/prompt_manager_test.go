package decision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptManager_LoadTemplates(t *testing.T) {
	// 创建临时目录用于测试
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupFiles    map[string]string // 文件名 -> 内容
		expectedCount int
		expectedNames []string
		shouldError   bool
	}{
		{
			name: "加载单个模板文件",
			setupFiles: map[string]string{
				"default.txt": "你是专业的加密货币交易AI。",
			},
			expectedCount: 1,
			expectedNames: []string{"default"},
			shouldError:   false,
		},
		{
			name: "加载多个模板文件",
			setupFiles: map[string]string{
				"default.txt":      "默认策略",
				"conservative.txt": "保守策略",
				"aggressive.txt":   "激进策略",
			},
			expectedCount: 3,
			expectedNames: []string{"default", "conservative", "aggressive"},
			shouldError:   false,
		},
		{
			name:          "空目录",
			setupFiles:    map[string]string{},
			expectedCount: 0,
			expectedNames: []string{},
			shouldError:   false,
		},
		{
			name: "忽略非.txt文件",
			setupFiles: map[string]string{
				"default.txt": "正确的模板",
				"readme.md":   "应该被忽略",
				"config.json": "应该被忽略",
			},
			expectedCount: 1,
			expectedNames: []string{"default"},
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个测试用例创建独立的子目录
			testDir := filepath.Join(tempDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("创建测试目录失败: %v", err)
			}

			// 设置测试文件
			for filename, content := range tt.setupFiles {
				filePath := filepath.Join(testDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("创建测试文件失败 %s: %v", filename, err)
				}
			}

			// 创建新的 PromptManager
			pm := NewPromptManager()

			// 执行测试
			err := pm.LoadTemplates(testDir)

			// 检查错误
			if (err != nil) != tt.shouldError {
				t.Errorf("LoadTemplates() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			// 检查加载的模板数量
			if len(pm.templates) != tt.expectedCount {
				t.Errorf("加载的模板数量 = %d, 期望 %d", len(pm.templates), tt.expectedCount)
			}

			// 检查模板名称
			for _, expectedName := range tt.expectedNames {
				if _, exists := pm.templates[expectedName]; !exists {
					t.Errorf("缺少预期的模板: %s", expectedName)
				}
			}

			// 验证模板内容
			for filename, expectedContent := range tt.setupFiles {
				if filepath.Ext(filename) != ".txt" {
					continue
				}
				templateName := filename[:len(filename)-4] // 去掉 .txt
				template, err := pm.GetTemplate(templateName)
				if err != nil {
					t.Errorf("获取模板 %s 失败: %v", templateName, err)
					continue
				}
				if template.Content != expectedContent {
					t.Errorf("模板内容不匹配\n期望: %s\n实际: %s", expectedContent, template.Content)
				}
			}
		})
	}
}

func TestPromptManager_GetTemplate(t *testing.T) {
	pm := NewPromptManager()
	pm.templates = map[string]*PromptTemplate{
		"default": {
			Name:    "default",
			Content: "默认策略内容",
		},
		"aggressive": {
			Name:    "aggressive",
			Content: "激进策略内容",
		},
	}

	tests := []struct {
		name            string
		templateName    string
		expectError     bool
		expectedContent string
	}{
		{
			name:            "获取存在的模板",
			templateName:    "default",
			expectError:     false,
			expectedContent: "默认策略内容",
		},
		{
			name:         "获取不存在的模板",
			templateName: "nonexistent",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := pm.GetTemplate(tt.templateName)

			if (err != nil) != tt.expectError {
				t.Errorf("GetTemplate() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && template.Content != tt.expectedContent {
				t.Errorf("模板内容 = %s, 期望 %s", template.Content, tt.expectedContent)
			}
		})
	}
}

func TestPromptManager_ReloadTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// 初始文件
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte("初始内容"), 0644); err != nil {
		t.Fatalf("创建初始文件失败: %v", err)
	}

	pm := NewPromptManager()
	if err := pm.LoadTemplates(tempDir); err != nil {
		t.Fatalf("初始加载失败: %v", err)
	}

	// 验证初始内容
	template, _ := pm.GetTemplate("default")
	if template.Content != "初始内容" {
		t.Errorf("初始内容不正确: %s", template.Content)
	}

	// 修改文件内容
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte("更新后内容"), 0644); err != nil {
		t.Fatalf("更新文件失败: %v", err)
	}

	// 添加新文件
	if err := os.WriteFile(filepath.Join(tempDir, "new.txt"), []byte("新模板内容"), 0644); err != nil {
		t.Fatalf("创建新文件失败: %v", err)
	}

	// 重新加载
	if err := pm.ReloadTemplates(tempDir); err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}

	// 验证更新后的内容
	template, err := pm.GetTemplate("default")
	if err != nil {
		t.Fatalf("获取 default 模板失败: %v", err)
	}
	if template.Content != "更新后内容" {
		t.Errorf("重新加载后内容不正确: got %s, want '更新后内容'", template.Content)
	}

	// 验证新模板
	newTemplate, err := pm.GetTemplate("new")
	if err != nil {
		t.Fatalf("获取 new 模板失败: %v", err)
	}
	if newTemplate.Content != "新模板内容" {
		t.Errorf("新模板内容不正确: %s", newTemplate.Content)
	}

	// 验证模板数量
	if len(pm.templates) != 2 {
		t.Errorf("重新加载后模板数量 = %d, 期望 2", len(pm.templates))
	}
}

func TestPromptManager_GetAllTemplateNames(t *testing.T) {
	pm := NewPromptManager()
	pm.templates = map[string]*PromptTemplate{
		"default":      {Name: "default", Content: "默认策略"},
		"conservative": {Name: "conservative", Content: "保守策略"},
		"aggressive":   {Name: "aggressive", Content: "激进策略"},
	}

	names := pm.GetAllTemplateNames()

	if len(names) != 3 {
		t.Errorf("GetAllTemplateNames() 返回数量 = %d, 期望 3", len(names))
	}

	// 验证所有名称都存在
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	expectedNames := []string{"default", "conservative", "aggressive"}
	for _, expectedName := range expectedNames {
		if !nameMap[expectedName] {
			t.Errorf("缺少预期的模板名称: %s", expectedName)
		}
	}
}

func TestReloadPromptTemplates_GlobalFunction(t *testing.T) {
	// 保存原始的 promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		// 恢复原始模板
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// 创建临时目录
	tempDir := t.TempDir()
	promptsDir = tempDir

	// 创建测试文件
	if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("测试内容"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 调用全局重新加载函数
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("ReloadPromptTemplates() 失败: %v", err)
	}

	// 验证全局管理器已更新
	template, err := GetPromptTemplate("test")
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}

	if template.Content != "测试内容" {
		t.Errorf("模板内容不正确: got %s, want '测试内容'", template.Content)
	}
}
