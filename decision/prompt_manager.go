package decision

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PromptTemplate system prompt template
type PromptTemplate struct {
	Name    string // Template name (filename without extension)
	Content string // Template content
}

// PromptManager prompt manager
type PromptManager struct {
	templates map[string]*PromptTemplate
	mu        sync.RWMutex
}

var (
	// globalPromptManager global prompt manager
	globalPromptManager *PromptManager
	// promptsDir prompt folder path
	promptsDir = "prompts"
)

// init loads all prompt templates during package initialization
func init() {
	globalPromptManager = NewPromptManager()
	if err := globalPromptManager.LoadTemplates(promptsDir); err != nil {
		log.Printf("⚠️  Failed to load prompt templates: %v", err)
	} else {
		log.Printf("✓ Loaded %d system prompt templates", len(globalPromptManager.templates))
	}
}

// NewPromptManager creates a prompt manager
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*PromptTemplate),
	}
}

// LoadTemplates loads all prompt templates from specified directory
func (pm *PromptManager) LoadTemplates(dir string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("prompt directory does not exist: %s", dir)
	}

	// Scan all .txt files in directory
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return fmt.Errorf("failed to scan prompt directory: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  No .txt files found in prompt directory %s", dir)
		return nil
	}

	// Load each template file
	for _, file := range files {
		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  Failed to read prompt file %s: %v", file, err)
			continue
		}

		// Extract filename (without extension) as template name
		fileName := filepath.Base(file)
		templateName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// Store template
		pm.templates[templateName] = &PromptTemplate{
			Name:    templateName,
			Content: string(content),
		}

		log.Printf("  📄 Loaded prompt template: %s (%s)", templateName, fileName)
	}

	return nil
}

// GetTemplate gets prompt template by name
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	template, exists := pm.templates[name]
	if !exists {
		return nil, fmt.Errorf("prompt template does not exist: %s", name)
	}

	return template, nil
}

// GetAllTemplateNames gets all template names list
func (pm *PromptManager) GetAllTemplateNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.templates))
	for name := range pm.templates {
		names = append(names, name)
	}

	return names
}

// GetAllTemplates gets all templates
func (pm *PromptManager) GetAllTemplates() []*PromptTemplate {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	templates := make([]*PromptTemplate, 0, len(pm.templates))
	for _, template := range pm.templates {
		templates = append(templates, template)
	}

	return templates
}

// ReloadTemplates reloads all templates
func (pm *PromptManager) ReloadTemplates(dir string) error {
	pm.mu.Lock()
	pm.templates = make(map[string]*PromptTemplate)
	pm.mu.Unlock()

	return pm.LoadTemplates(dir)
}

// === Global functions (for external calls) ===

// GetPromptTemplate gets prompt template by name (global function)
func GetPromptTemplate(name string) (*PromptTemplate, error) {
	return globalPromptManager.GetTemplate(name)
}

// GetAllPromptTemplateNames gets all template names (global function)
func GetAllPromptTemplateNames() []string {
	return globalPromptManager.GetAllTemplateNames()
}

// GetAllPromptTemplates gets all templates (global function)
func GetAllPromptTemplates() []*PromptTemplate {
	return globalPromptManager.GetAllTemplates()
}

// ReloadPromptTemplates reloads all templates (global function)
func ReloadPromptTemplates() error {
	return globalPromptManager.ReloadTemplates(promptsDir)
}
