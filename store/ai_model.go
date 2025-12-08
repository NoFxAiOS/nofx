package store

import (
	"database/sql"
	"errors"
	"fmt"
	"nofx/logger"
	"strings"
	"time"
)

// AIModelStore AI model storage
type AIModelStore struct {
	db            *sql.DB
	encryptFunc   func(string) string
	decryptFunc   func(string) string
}

// AIModel AI model configuration
type AIModel struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (s *AIModelStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			custom_api_url TEXT DEFAULT '',
			custom_model_name TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Trigger
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
		AFTER UPDATE ON ai_models
		BEGIN
			UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return err
	}

	// Backward compatibility: add potentially missing columns
	s.db.Exec(`ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`)
	s.db.Exec(`ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`)

	return nil
}

func (s *AIModelStore) initDefaultData() error {
	// No longer pre-populate AI models - create on demand when user configures
	return nil
}

func (s *AIModelStore) encrypt(plaintext string) string {
	if s.encryptFunc != nil {
		return s.encryptFunc(plaintext)
	}
	return plaintext
}

func (s *AIModelStore) decrypt(encrypted string) string {
	if s.decryptFunc != nil {
		return s.decryptFunc(encrypted)
	}
	return encrypted
}

// List retrieves user's AI model list
func (s *AIModelStore) List(userID string) ([]*AIModel, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, '') as custom_api_url,
		       COALESCE(custom_model_name, '') as custom_model_name,
		       created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]*AIModel, 0)
	for rows.Next() {
		var model AIModel
		var createdAt, updatedAt string
		err := rows.Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		model.APIKey = s.decrypt(model.APIKey)
		models = append(models, &model)
	}
	return models, nil
}

// Get retrieves a single AI model
func (s *AIModelStore) Get(userID, modelID string) (*AIModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	candidates := []string{}
	if userID != "" {
		candidates = append(candidates, userID)
	}
	if userID != "default" {
		candidates = append(candidates, "default")
	}
	if len(candidates) == 0 {
		candidates = append(candidates, "default")
	}

	for _, uid := range candidates {
		var model AIModel
		var createdAt, updatedAt string
		err := s.db.QueryRow(`
			SELECT id, user_id, name, provider, enabled, api_key,
			       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
			FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1
		`, uid, modelID).Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&createdAt, &updatedAt,
		)
		if err == nil {
			model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
			model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
			model.APIKey = s.decrypt(model.APIKey)
			return &model, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return nil, sql.ErrNoRows
}

// GetDefault retrieves the default enabled AI model
func (s *AIModelStore) GetDefault(userID string) (*AIModel, error) {
	if userID == "" {
		userID = "default"
	}
	model, err := s.firstEnabled(userID)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if userID != "default" {
		return s.firstEnabled("default")
	}
	return nil, fmt.Errorf("please configure an available AI model in the system first")
}

func (s *AIModelStore) firstEnabled(userID string) (*AIModel, error) {
	var model AIModel
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
		FROM ai_models WHERE user_id = ? AND enabled = 1
		ORDER BY datetime(updated_at) DESC, id ASC LIMIT 1
	`, userID).Scan(
		&model.ID, &model.UserID, &model.Name, &model.Provider,
		&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	model.APIKey = s.decrypt(model.APIKey)
	return &model, nil
}

// Update updates AI model, creates if not exists
// IMPORTANT: If apiKey is empty string, the existing API key will be preserved (not overwritten)
func (s *AIModelStore) Update(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// Try exact ID match first
	var existingID string
	err := s.db.QueryRow(`SELECT id FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1`, userID, id).Scan(&existingID)
	if err == nil {
		// If apiKey is empty, preserve the existing API key
		if apiKey == "" {
			_, err = s.db.Exec(`
				UPDATE ai_models SET enabled = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, enabled, customAPIURL, customModelName, existingID, userID)
		} else {
			encryptedAPIKey := s.encrypt(apiKey)
			_, err = s.db.Exec(`
				UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		}
		return err
	}

	// Try legacy logic compatibility: use id as provider to search
	provider := id
	err = s.db.QueryRow(`SELECT id FROM ai_models WHERE user_id = ? AND provider = ? LIMIT 1`, userID, provider).Scan(&existingID)
	if err == nil {
		logger.Warnf("⚠️ Using legacy provider matching to update model: %s -> %s", provider, existingID)
		// If apiKey is empty, preserve the existing API key
		if apiKey == "" {
			_, err = s.db.Exec(`
				UPDATE ai_models SET enabled = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, enabled, customAPIURL, customModelName, existingID, userID)
		} else {
			encryptedAPIKey := s.encrypt(apiKey)
			_, err = s.db.Exec(`
				UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
				WHERE id = ? AND user_id = ?
			`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		}
		return err
	}

	// Create new record
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		provider = id
	} else {
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1]
		} else {
			provider = id
		}
	}

	var name string
	err = s.db.QueryRow(`SELECT name FROM ai_models WHERE provider = ? LIMIT 1`, provider).Scan(&name)
	if err != nil {
		if provider == "deepseek" {
			name = "DeepSeek AI"
		} else if provider == "qwen" {
			name = "Qwen AI"
		} else {
			name = provider + " AI"
		}
	}

	newModelID := id
	if id == provider {
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	logger.Infof("✓ Creating new AI model configuration: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := s.encrypt(apiKey)
	_, err = s.db.Exec(`
		INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)
	return err
}

// Create creates an AI model
func (s *AIModelStore) Create(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, provider, enabled, apiKey, customAPIURL)
	return err
}
