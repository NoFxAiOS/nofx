package decision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPromptReloadEndToEnd end-to-end test: verify complete flow from file modification to decision engine usage
func TestPromptReloadEndToEnd(t *testing.T) {
	// Save original promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		// Restore original templates
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// Create temporary directory to simulate prompts/ directory
	tempDir := t.TempDir()
	promptsDir = tempDir

	// Step 1: Create initial prompt file
	initialContent := "# Initial Trading Strategy\nYou are a conservative trading AI."
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Step 2: First load (simulate system startup)
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("First load failed: %v", err)
	}

	// Step 3: Verify initial content
	template, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("Failed to get initial template: %v", err)
	}
	if template.Content != initialContent {
		t.Errorf("Initial content mismatch\nExpected: %s\nActual: %s", initialContent, template.Content)
	}

	// Step 4: Use buildSystemPrompt to verify template is correctly used
	systemPrompt := buildSystemPrompt(10000.0, 10, 5, "test_strategy", "")
	if !strings.Contains(systemPrompt, initialContent) {
		t.Errorf("buildSystemPrompt doesn't contain template content\nGenerated prompt:\n%s", systemPrompt)
	}

	// Step 5: Simulate user modifying file (user modifies prompt on disk)
	updatedContent := "# Updated Trading Strategy\nYou are an aggressive trading AI seeking high risk and high reward."
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(updatedContent), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	// Step 6: Simulate trader startup calling ReloadPromptTemplates()
	t.Log("Simulating trader startup, calling ReloadPromptTemplates()...")
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Step 7: Verify new content has taken effect
	reloadedTemplate, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("Failed to get reloaded template: %v", err)
	}
	if reloadedTemplate.Content != updatedContent {
		t.Errorf("Content mismatch after reload\nExpected: %s\nActual: %s", updatedContent, reloadedTemplate.Content)
	}

	// Step 8: Verify buildSystemPrompt uses new content
	newSystemPrompt := buildSystemPrompt(10000.0, 10, 5, "test_strategy", "")
	if !strings.Contains(newSystemPrompt, updatedContent) {
		t.Errorf("buildSystemPrompt doesn't contain updated template content\nGenerated prompt:\n%s", newSystemPrompt)
	}

	// Step 9: Verify old content no longer exists
	if strings.Contains(newSystemPrompt, "conservative trading AI") {
		t.Errorf("buildSystemPrompt still contains old template content")
	}

	t.Log("✅ End-to-end test passed: file modification -> reload -> decision engine uses new content")
}

// TestPromptReloadWithCustomPrompt tests interaction between custom prompt and template reload
func TestPromptReloadWithCustomPrompt(t *testing.T) {
	// Save original promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	promptsDir = tempDir

	// Create base template
	baseContent := "Base strategy: Stable trading"
	if err := os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte(baseContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Load templates
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test 1: Base template + custom prompt (no override)
	customPrompt := "Personalized rule: Only trade BTC"
	result := buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, false, "base", "")
	if !strings.Contains(result, baseContent) {
		t.Errorf("Doesn't contain base template content")
	}
	if !strings.Contains(result, customPrompt) {
		t.Errorf("Doesn't contain custom prompt")
	}

	// Test 2: Override base prompt
	result = buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, true, "base", "")
	if strings.Contains(result, baseContent) {
		t.Errorf("Override mode still contains base template content")
	}
	if !strings.Contains(result, customPrompt) {
		t.Errorf("Override mode doesn't contain custom prompt")
	}

	// Test 3: Effect after reload
	updatedBase := "Updated base strategy: Aggressive trading"
	if err := os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte(updatedBase), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	result = buildSystemPromptWithCustom(10000.0, 10, 5, customPrompt, false, "base", "")
	if !strings.Contains(result, updatedBase) {
		t.Errorf("After reload doesn't contain updated base template content")
	}
	if strings.Contains(result, baseContent) {
		t.Errorf("After reload still contains old base template content")
	}
}

// TestPromptReloadFallback tests fallback mechanism when template doesn't exist
func TestPromptReloadFallback(t *testing.T) {
	// Save original promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	promptsDir = tempDir

	// Only create default template
	defaultContent := "Default strategy"
	if err := os.WriteFile(filepath.Join(tempDir, "default.txt"), []byte(defaultContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test 1: Request non-existent template, should fall back to default
	result := buildSystemPrompt(10000.0, 10, 5, "nonexistent", "")
	if !strings.Contains(result, defaultContent) {
		t.Errorf("When requesting non-existent template, didn't fall back to default")
	}

	// Test 2: Empty template name, should use default
	result = buildSystemPrompt(10000.0, 10, 5, "", "")
	if !strings.Contains(result, defaultContent) {
		t.Errorf("With empty template name, didn't use default")
	}
}

// TestConcurrentPromptReload tests prompt reload in concurrent scenarios
func TestConcurrentPromptReload(t *testing.T) {
	// Save original promptsDir
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	promptsDir = tempDir

	// Create test file
	if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Test content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("Initial load failed: %v", err)
	}

	// Concurrent test: read and reload simultaneously
	done := make(chan bool)

	// Start multiple read goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = GetPromptTemplate("test")
			}
			done <- true
		}()
	}

	// Start multiple reload goroutines
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = ReloadPromptTemplates()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 13; i++ {
		<-done
	}

	// Verify final state is correct
	template, err := GetPromptTemplate("test")
	if err != nil {
		t.Errorf("Failed to get template after concurrent test: %v", err)
	}
	if template.Content != "Test content" {
		t.Errorf("Template content error after concurrent test: %s", template.Content)
	}

	t.Log("✅ Concurrent test passed: multiple goroutines reading and reloading templates simultaneously, no data race")
}
