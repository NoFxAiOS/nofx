// Package news API处理层 - 用户新闻源配置
// 设计哲学：清晰的输入输出、统一的错误处理、完整的参数验证
package api

import (
	"fmt"
	"net/http"
	"nofx/database"
	"strings"

	"github.com/gin-gonic/gin"
)

// NewsConfigHandler 用户新闻配置API处理器
type NewsConfigHandler struct {
	repo database.NewsConfigRepository
}

// NewNewsConfigHandler 创建用户新闻配置API处理器
func NewNewsConfigHandler(repo database.NewsConfigRepository) *NewsConfigHandler {
	return &NewsConfigHandler{repo: repo}
}

// 注意：路由注册应该在 api/server.go 中通过 Server.authMiddleware() 进行
// 不要在handler中定义私有中间件

// GetUserNewsConfigRequest 获取用户新闻配置请求
type GetUserNewsConfigResponse struct {
	ID                      int      `json:"id"`
	UserID                  string   `json:"user_id"`
	Enabled                 bool     `json:"enabled"`
	NewsSources             string   `json:"news_sources"`
	NewSourcesList          []string `json:"news_sources_list"`
	AutoFetchIntervalMinutes int      `json:"auto_fetch_interval_minutes"`
	MaxArticlesPerFetch     int      `json:"max_articles_per_fetch"`
	SentimentThreshold      float64  `json:"sentiment_threshold"`
	CreatedAt               int64    `json:"created_at"`
	UpdatedAt               int64    `json:"updated_at"`
}

// CreateOrUpdateUserNewsConfigRequest 创建或更新用户新闻配置请求
type CreateOrUpdateUserNewsConfigRequest struct {
	Enabled                 *bool    `json:"enabled"`
	NewsSources             *string  `json:"news_sources"`
	AutoFetchIntervalMinutes *int     `json:"auto_fetch_interval_minutes"`
	MaxArticlesPerFetch     *int     `json:"max_articles_per_fetch"`
	SentimentThreshold      *float64 `json:"sentiment_threshold"`
}

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// GetUserNewsConfig 获取用户新闻配置
// @Summary 获取用户新闻配置
// @Description 获取当前用户的新闻源配置
// @Tags News
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse "未授权"
// @Failure 404 {object} APIResponse "配置不存在"
// @Router /api/user/news-config [get]
func (h *NewsConfigHandler) GetUserNewsConfig(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Code:    401,
			Message: "未授权的访问",
		})
		return
	}

	config, err := h.repo.GetByUserID(userID)
	if err != nil {
		// 如果配置不存在，返回404
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "用户新闻配置不存在",
		})
		return
	}

	response := GetUserNewsConfigResponse{
		ID:                       config.ID,
		UserID:                   config.UserID,
		Enabled:                  config.Enabled,
		NewsSources:              config.NewsSources,
		NewSourcesList:           config.GetEnabledNewsSources(),
		AutoFetchIntervalMinutes: config.AutoFetchIntervalMinutes,
		MaxArticlesPerFetch:      config.MaxArticlesPerFetch,
		SentimentThreshold:       config.SentimentThreshold,
		CreatedAt:                config.CreatedAt.Unix(),
		UpdatedAt:                config.UpdatedAt.Unix(),
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "获取配置成功",
		Data:    response,
	})
}

// CreateOrUpdateUserNewsConfig 创建或更新用户新闻配置
// @Summary 创建或更新用户新闻配置
// @Description 创建新的用户新闻配置或更新现有配置
// @Tags News
// @Accept json
// @Produce json
// @Param request body CreateOrUpdateUserNewsConfigRequest true "配置信息"
// @Success 201 {object} APIResponse
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 401 {object} APIResponse "未授权"
// @Router /api/user/news-config [post]
func (h *NewsConfigHandler) CreateOrUpdateUserNewsConfig(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Code:    401,
			Message: "未授权的访问",
		})
		return
	}

	var req CreateOrUpdateUserNewsConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 尝试获取现有配置
	config, err := h.repo.GetByUserID(userID)
	if err != nil {
		// 配置不存在，创建新的
		config = &database.UserNewsConfig{
			UserID:                  userID,
			Enabled:                 true,
			NewsSources:             "mlion",
			AutoFetchIntervalMinutes: 5,
			MaxArticlesPerFetch:     10,
			SentimentThreshold:      0.0,
		}
	}

	// 应用请求中的更新
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.NewsSources != nil {
		// 验证新闻源
		sources := strings.Split(*req.NewsSources, ",")
		validSources := make([]string, 0)
		for _, source := range sources {
			source = strings.TrimSpace(source)
			if source != "" && isValidNewsSource(source) {
				validSources = append(validSources, source)
			}
		}
		if len(validSources) == 0 {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "必须至少指定一个有效的新闻源",
			})
			return
		}
		config.NewsSources = strings.Join(validSources, ",")
	}
	if req.AutoFetchIntervalMinutes != nil {
		if *req.AutoFetchIntervalMinutes < 1 || *req.AutoFetchIntervalMinutes > 1440 {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "抓取间隔必须在1-1440分钟之间",
			})
			return
		}
		config.AutoFetchIntervalMinutes = *req.AutoFetchIntervalMinutes
	}
	if req.MaxArticlesPerFetch != nil {
		if *req.MaxArticlesPerFetch < 1 || *req.MaxArticlesPerFetch > 100 {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "每次抓取的最大文章数必须在1-100之间",
			})
			return
		}
		config.MaxArticlesPerFetch = *req.MaxArticlesPerFetch
	}
	if req.SentimentThreshold != nil {
		if *req.SentimentThreshold < -1.0 || *req.SentimentThreshold > 1.0 {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "情绪阈值必须在-1.0到1.0之间",
			})
			return
		}
		config.SentimentThreshold = *req.SentimentThreshold
	}

	// 保存配置
	if config.ID == 0 {
		// 创建新配置
		err = h.repo.Create(config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: fmt.Sprintf("创建配置失败: %v", err),
			})
			return
		}
		c.JSON(http.StatusCreated, APIResponse{
			Code:    201,
			Message: "配置创建成功",
			Data: GetUserNewsConfigResponse{
				ID:                       config.ID,
				UserID:                   config.UserID,
				Enabled:                  config.Enabled,
				NewsSources:              config.NewsSources,
				NewSourcesList:           config.GetEnabledNewsSources(),
				AutoFetchIntervalMinutes: config.AutoFetchIntervalMinutes,
				MaxArticlesPerFetch:      config.MaxArticlesPerFetch,
				SentimentThreshold:       config.SentimentThreshold,
				CreatedAt:                config.CreatedAt.Unix(),
				UpdatedAt:                config.UpdatedAt.Unix(),
			},
		})
	} else {
		// 更新现有配置
		err = h.repo.Update(config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: fmt.Sprintf("更新配置失败: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, APIResponse{
			Code:    200,
			Message: "配置更新成功",
			Data: GetUserNewsConfigResponse{
				ID:                       config.ID,
				UserID:                   config.UserID,
				Enabled:                  config.Enabled,
				NewsSources:              config.NewsSources,
				NewSourcesList:           config.GetEnabledNewsSources(),
				AutoFetchIntervalMinutes: config.AutoFetchIntervalMinutes,
				MaxArticlesPerFetch:      config.MaxArticlesPerFetch,
				SentimentThreshold:       config.SentimentThreshold,
				CreatedAt:                config.CreatedAt.Unix(),
				UpdatedAt:                config.UpdatedAt.Unix(),
			},
		})
	}
}

// UpdateUserNewsConfig 更新用户新闻配置（PUT方法）
func (h *NewsConfigHandler) UpdateUserNewsConfig(c *gin.Context) {
	// PUT方法与POST逻辑相同
	h.CreateOrUpdateUserNewsConfig(c)
}

// DeleteUserNewsConfig 删除用户新闻配置
// @Summary 删除用户新闻配置
// @Description 删除当前用户的新闻源配置
// @Tags News
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse "未授权"
// @Failure 404 {object} APIResponse "配置不存在"
// @Router /api/user/news-config [delete]
func (h *NewsConfigHandler) DeleteUserNewsConfig(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Code:    401,
			Message: "未授权的访问",
		})
		return
	}

	err := h.repo.Delete(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "用户新闻配置不存在",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "配置删除成功",
	})
}

// GetEnabledNewsSources 获取启用的新闻源列表
// @Summary 获取启用的新闻源
// @Description 获取当前用户启用的新闻源列表
// @Tags News
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse "未授权"
// @Router /api/user/news-config/sources [get]
func (h *NewsConfigHandler) GetEnabledNewsSources(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Code:    401,
			Message: "未授权的访问",
		})
		return
	}

	config, err := h.repo.GetByUserID(userID)
	if err != nil {
		// 如果配置不存在，返回默认的
		c.JSON(http.StatusOK, APIResponse{
			Code:    200,
			Message: "获取新闻源成功",
			Data: map[string]interface{}{
				"sources": []string{"mlion"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    200,
		Message: "获取新闻源成功",
		Data: map[string]interface{}{
			"sources": config.GetEnabledNewsSources(),
		},
	})
}

// ===== 辅助函数 =====

// getUserID 从context中获取用户ID
// user_id由Server.authMiddleware()在认证时设置
func getUserID(c *gin.Context) string {
	return c.GetString("user_id")
}

// isValidNewsSource 检查新闻源是否有效
func isValidNewsSource(source string) bool {
	validSources := map[string]bool{
		"mlion":   true,
		"twitter": true,
		"reddit":  true,
		"telegram": true,
	}
	return validSources[strings.ToLower(source)]
}
