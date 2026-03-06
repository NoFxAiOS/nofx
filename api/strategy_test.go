package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"nofx/store"
)

func TestHandlePreviewPrompt_MacroMicro_ReturnsSteps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := store.GetDefaultStrategyConfig("en")
	config.EnableMacroMicroFlow = true

	body := map[string]interface{}{
		"config":          config,
		"account_equity":  1000.0,
		"prompt_variant":  "balanced",
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/strategies/preview-prompt", bytes.NewBuffer(bodyJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_id", "test-user")

	s := &Server{}
	s.handlePreviewPrompt(c)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, hasSteps := resp["steps"]; !hasSteps {
		t.Errorf("Expected response to have 'steps' array, got keys: %v", keys(resp))
	}

	if _, hasSystemPrompt := resp["system_prompt"]; hasSystemPrompt {
		t.Errorf("Macro-micro response should NOT have top-level 'system_prompt'")
	}

	steps, ok := resp["steps"].([]interface{})
	if !ok {
		t.Fatalf("'steps' is not an array")
	}

	stepLabels := make(map[string]bool)
	for _, s := range steps {
		m, ok := s.(map[string]interface{})
		if !ok {
			t.Errorf("Step is not an object: %v", s)
			continue
		}
		if step, _ := m["step"].(string); step != "" {
			stepLabels[step] = true
		}
	}

	for _, want := range []string{"macro", "deep_dive", "position_check"} {
		if !stepLabels[want] {
			t.Errorf("Expected steps to include %q, got: %v", want, stepLabels)
		}
	}
}

func TestHandlePreviewPrompt_SingleTurn_ReturnsSystemPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := store.GetDefaultStrategyConfig("en")
	config.EnableMacroMicroFlow = false

	body := map[string]interface{}{
		"config":          config,
		"account_equity":  1000.0,
		"prompt_variant":  "balanced",
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/strategies/preview-prompt", bytes.NewBuffer(bodyJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_id", "test-user")

	s := &Server{}
	s.handlePreviewPrompt(c)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, hasSystemPrompt := resp["system_prompt"]; !hasSystemPrompt {
		t.Errorf("Single-turn response should have 'system_prompt', got keys: %v", keys(resp))
	}

	if _, hasSteps := resp["steps"]; hasSteps {
		t.Errorf("Single-turn response should NOT have 'steps'")
	}
}

func keys(m map[string]interface{}) []string {
	var k []string
	for kk := range m {
		k = append(k, kk)
	}
	return k
}
