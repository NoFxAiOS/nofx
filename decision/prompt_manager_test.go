package decision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptManager_LoadTemplates(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupFiles    map[string]string // filename -> content
		expectedCount int
		expectedNames []string
		shouldError   bool
	}{
		{
			name: "Load single template file",
			setupFiles: map[string]string{
				"default.txt": "You are a professional cryptocurrency trading AI.",
			},
			expectedCount: 1,
			expectedNames: []string{"default"},
			shouldError:   false,
		},
		{
			name: "Load multiple template files",
			setupFiles: map[string]string{
				"default.txt":      "Default strategy",
				"conservative.txt": "Conservative strategy",
				"aggressive.txt":   "Aggressive strategy",
			},
			expectedCount: 3,
			expectedNames: []string{"default", "conservative", "aggressive"},
			shouldError:   false,
		},
		{
			name:          "Empty directory",
			setupFiles:    map[string]string{},
			expectedCount: 0,
			expectedNames: []string{},
			shouldError:   false,
		},
		{
			name: "Ignore non-.txt files",
			setupFiles: map[string]string{
				"default.txt": "Correct template",
				"readme.md":   "Should be ignored",
				"config.json": "Should be ignored",
			},
			expectedCount: 1,
			expectedNames: []string{"default"},
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create independent subdirectory for each test case
			testDir := filepath.Join(tempDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Setup test files
			for filename, content := range tt.setupFiles {
				filePath := filepath.Join(testDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			// Create new PromptManager
			pm := NewPromptManager()

			// Execute test
			err := pm.LoadTemplates(testDir)

			// Check error
			if (err != nil) != tt.shouldError {
				t.Errorf("LoadTemplates() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			// Check loaded template count
			if len(pm.templates) != tt.expectedCount {
				t.Errorf("Loaded template count = %d, expected %d", len(pm.templates), tt.expectedCount)
			}

			// Check template names
			for _, expectedName := range tt.expectedNames {
				if _, exists := pm.templates[expectedName]; !exists {
					t.Errorf("Missing expected template: %s", expectedName)
				}
			}

			// Verify template content
			for filename, expectedContent := range tt.setupFiles {
				if filepath.Ext(filename) != ".txt" {
					continue
				}
				templateName := filename[:len(filename)-4] // Remove .txt
				template, err := pm.GetTemplate(templateName)
				if err != nil {
					t.Errorf("Failed to get template %s: %v", templateName, err)
					continue
				}
				if template.Content != expectedContent {
					t.Errorf("Template content mismatch\nExpected: %s\nActual: %s", expectedContent, template.Content)
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
			Content: "Default strategy content",
		},
		"aggressive": {
			Name:    "aggressive",
			Content: "Aggressive strategy content",
		},
	}

	tests := []struct {
		name            string
		templateName    string
		expectError     bool
		expectedContent string
	}{
		{
			name:            "Get existing template",
			templateName:    "default",
			expectError:     false,
			expectedContent: "Default strategy content",
		},
		{
			name:         "Get non-existent template",
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
				t.Errorf("Template content = %s, expected %s", template.Content, tt.expectedContent)
			}
		})
	}
}

func TestPromptManager_ReloadTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// Initial file
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte("Initial content"), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	pm := NewPromptManager()
	if err := pm.LoadTemplates(tempDir); err != nil {
		t.Fatalf("Initial load failed: %v", err)
	}

	// Verify initial content
	template, _ := pm.GetTemplate("default")
	if template.Content != "Initial content" {
		t.Errorf("Initial content incorrect: %s", template.Content)
	}

	// Modify file content
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte("Updated content"), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	// Add new file
	if err := os.WriteFile(filepath.Join(tempDir, "new.txt"), []byte("New template content"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Reload
	if err := pm.ReloadTemplates(tempDir); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify updated content
	template, err := pm.GetTemplate("default")
	if err != nil {
		t.Fatalf("Failed to get default template: %v", err)
	}
	if template.Content != "Updated content" {
		t.Errorf("Content after reload incorrect: got %s, want 'Updated content'", template.Content)
	}

	// Verify new template
	newTemplate, err := pm.GetTemplate("new")
	if err != nil {
		t.Fatalf("Failed to get new template: %v", err)
	}
	if newTemplate.Content != "New template content" {
		t.Errorf("New template content incorrect: %s", newTemplate.Content)
	}

	// Verify template count
	if len(pm.templates) != 2 {
		t.Errorf("Template count after reload = %d, expected 2", len(pm.templates))
	}
}

func TestPromptManager_GetAllTemplateNames(t *testing.T) {
	pm := NewPromptManager()
	pm.templates = map[string]*PromptTemplate{
		"default":      {Name: "default", Content: "Default strategy"},
		"conservative": {Name: "conservative", Content: "Conservative strategy"},
		"aggressive":   {Name: "aggressive", Content: "Aggressive strategy"},
	}

	names := pm.GetAllTemplateNames()

	if len(names) != 3 {
		t.Errorf("GetAllTemplateNames() returned count = %d, expected 3", len(names))
	}

	// Verify all names exist
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	expectedNames := []string{"default", "conservative", "aggressive"}
	for _, expectedName := range expectedNames {
		if !nameMap[expectedName] {
			t.Errorf("Missing expected template name: %s", expectedName)
		}
	}
}

func TestReloadPromptTemplates_GlobalFunction(t *testing.T) {
	// Save original promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		// Restore original templates
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	promptsDir = tempDir

	// Create test file
	if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Call global reload function
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("ReloadPromptTemplates() failed: %v", err)
	}

	// Verify global manager has been updated
	template, err := GetPromptTemplate("test")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if template.Content != "Test content" {
		t.Errorf("Template content incorrect: got %s, want 'Test content'", template.Content)
	}
}
