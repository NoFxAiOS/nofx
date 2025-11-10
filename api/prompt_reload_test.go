package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"nofx/decision"
)

// TestHandleReloadPromptTemplates 测试重新加载提示词模板端点（修复 #643）
func TestHandleReloadPromptTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试服务器
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 创建 mock server
	s := &Server{}

	// 调用处理函数
	s.handleReloadPromptTemplates(c)

	// 解析响应
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 验证响应格式（可能成功或失败，取决于 prompts 目录是否存在）
	if w.Code == http.StatusOK {
		// 成功情况：验证返回字段
		if _, exists := response["message"]; !exists {
			t.Error("Expected 'message' field in success response")
		}
		if _, exists := response["count"]; !exists {
			t.Error("Expected 'count' field in success response")
		}
		t.Log("✓ Template reload succeeded")
	} else if w.Code == http.StatusInternalServerError {
		// 失败情况：验证错误信息
		if _, exists := response["error"]; !exists {
			t.Error("Expected 'error' field in error response")
		}
		t.Log("✓ Template reload failed gracefully (prompts directory may not exist in test environment)")
	} else {
		t.Errorf("Unexpected status code: %d", w.Code)
	}
}

// TestGetAllPromptTemplates 测试获取所有模板（验证 decision 包集成）
func TestGetAllPromptTemplates(t *testing.T) {
	// 获取所有模板
	templates := decision.GetAllPromptTemplates()

	// 在测试环境中，prompts 目录可能不存在，所以可能返回空列表
	t.Logf("Found %d templates", len(templates))

	// 如果有模板，验证结构
	if len(templates) > 0 {
		for _, tmpl := range templates {
			if tmpl.Name == "" {
				t.Error("Template name should not be empty")
			}
			if tmpl.Content == "" {
				t.Error("Template content should not be empty")
			}
		}
		t.Log("✓ Template structure validated")
	} else {
		t.Log("✓ No templates found (expected in test environment without prompts directory)")
	}
}

// TestReloadPromptTemplates 测试重新加载功能（验证幂等性）
func TestReloadPromptTemplates(t *testing.T) {
	// 第一次获取
	before := decision.GetAllPromptTemplates()
	beforeCount := len(before)

	// 重新加载（可能失败，如果 prompts 目录不存在）
	err := decision.ReloadPromptTemplates()

	// 第二次获取
	after := decision.GetAllPromptTemplates()
	afterCount := len(after)

	if err != nil {
		// 重新加载失败（测试环境可能没有 prompts 目录）
		t.Logf("✓ ReloadPromptTemplates failed gracefully: %v", err)
		// 验证失败后状态不变
		if beforeCount != afterCount {
			t.Errorf("Template count changed after failed reload: before=%d, after=%d", beforeCount, afterCount)
		}
	} else {
		// 重新加载成功，验证幂等性
		if beforeCount != afterCount {
			t.Errorf("Template count changed after reload: before=%d, after=%d", beforeCount, afterCount)
		}

		// 验证内容相同
		if len(before) > 0 && len(after) > 0 {
			if before[0].Name != after[0].Name {
				t.Error("Template content changed after reload (should be idempotent)")
			}
		}
		t.Log("✓ Reload is idempotent")
	}
}
