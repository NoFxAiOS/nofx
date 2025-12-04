// Package credits 积分系统API处理层
// 设计哲学：清晰的输入输出、统一的错误处理、完整的参数验证
package credits

import (
        "fmt"
        "net/http"
        "nofx/config"
        "nofx/service/credits"
        "strconv"
        "strings"

        "github.com/gin-gonic/gin"
)

// Handler 积分API处理器
type Handler struct {
        service credits.Service
}

// NewHandler 创建积分API处理器
func NewHandler(service credits.Service) *Handler {
        return &Handler{service: service}
}

// RegisterRoutes 注册积分相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
        // 公开的套餐查询接口（无需认证）
        router.GET("/credit-packages", h.HandleGetCreditPackages)
        router.GET("/credit-packages/:id", h.HandleGetCreditPackage)

        // 需要认证的用户积分接口
        protected := router.Group("/")
        protected.Use(authMiddleware())
        {
                protected.GET("/user/credits", h.HandleGetUserCredits)
                protected.GET("/user/credits/transactions", h.HandleGetUserTransactions)
                protected.GET("/user/credits/summary", h.HandleGetUserCreditSummary)
        }

        // 管理员接口
        admin := router.Group("/admin/")
        admin.Use(authMiddleware())
        admin.Use(adminMiddleware())
        {
                // 套餐管理
                admin.POST("/credit-packages", h.HandleCreateCreditPackage)
                admin.PUT("/credit-packages/:id", h.HandleUpdateCreditPackage)
                admin.DELETE("/credit-packages/:id", h.HandleDeleteCreditPackage)

                // 用户积分管理
                admin.POST("/users/:id/credits/adjust", h.HandleAdjustUserCredits)
                admin.GET("/users/:id/credits", h.HandleGetUserCreditsByAdmin)
                admin.GET("/users/:id/credits/transactions", h.HandleGetUserTransactionsByAdmin)
        }
}

// handleGetCreditPackages 获取积分套餐列表
// @Summary 获取所有启用的积分套餐
// @Description 获取系统中所有可用的积分套餐，按排序顺序返回
// @Tags 积分系统
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "套餐列表"
// @Failure 500 {object} map[string]string "错误信息"
// @Router /api/v1/credit-packages [get]
func (h *Handler) HandleGetCreditPackages(c *gin.Context) {
        packages, err := h.service.GetActivePackages(c.Request.Context())
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取套餐列表失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "packages": packages,
                        "total": len(packages),
                },
        })
}

// handleGetCreditPackage 获取指定积分套餐
// @Summary 获取积分套餐详情
// @Description 根据套餐ID获取详细信息
// @Tags 积分系统
// @Accept json
// @Produce json
// @Param id path string true "套餐ID"
// @Success 200 {object} map[string]interface{} "套餐详情"
// @Failure 404 {object} map[string]string "套餐不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/credit-packages/{id} [get]
func (h *Handler) HandleGetCreditPackage(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "套餐ID不能为空",
                })
                return
        }

        pkg, err := h.service.GetPackageByID(c.Request.Context(), id)
        if err != nil {
                if strings.Contains(err.Error(), "no rows") {
                        c.JSON(http.StatusNotFound, gin.H{
                                "error": "套餐不存在",
                        })
                        return
                }
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取套餐失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "package": pkg,
                },
        })
}

// handleGetUserCredits 获取用户积分余额
// @Summary 获取当前用户积分余额
// @Description 获取用户的可用积分、总积分和已用积分
// @Tags 积分系统
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "积分信息"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/user/credits [get]
func (h *Handler) HandleGetUserCredits(c *gin.Context) {
        userID := getUserID(c)
        if userID == "" {
                c.JSON(http.StatusUnauthorized, gin.H{
                        "error": "用户未认证",
                })
                return
        }

        credits, err := h.service.GetUserCredits(c.Request.Context(), userID)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取积分失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "available_credits": credits.AvailableCredits,
                        "total_credits": credits.TotalCredits,
                        "used_credits": credits.UsedCredits,
                },
        })
}

// handleGetUserTransactions 获取用户积分流水
// @Summary 获取用户积分交易记录
// @Description 获取用户的积分收入和支出记录，支持分页
// @Tags 积分系统
// @Accept json
// @Produce json
// @Param page query int false "页码，默认为1"
// @Param limit query int false "每页数量，默认为20，最大100"
// @Success 200 {object} map[string]interface{} "流水列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/user/credits/transactions [get]
func (h *Handler) HandleGetUserTransactions(c *gin.Context) {
        userID := getUserID(c)
        if userID == "" {
                c.JSON(http.StatusUnauthorized, gin.H{
                        "error": "用户未认证",
                })
                return
        }

        // 解析分页参数
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

        // 参数验证
        if page < 1 {
                page = 1
        }
        if limit < 1 || limit > 100 {
                limit = 20
        }

        transactions, total, err := h.service.GetUserTransactions(c.Request.Context(), userID, page, limit)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取流水失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "transactions": transactions,
                        "total": total,
                        "page": page,
                        "limit": limit,
                        "total_pages": (total + limit - 1) / limit,
                },
        })
}

// handleGetUserCreditSummary 获取用户积分摘要
// @Summary 获取用户积分统计摘要
// @Description 获取用户的积分统计信息，包括本月消费、充值等
// @Tags 积分系统
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "积分摘要"
// @Failure 401 {object} map[string]string "未认证"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/user/credits/summary [get]
func (h *Handler) HandleGetUserCreditSummary(c *gin.Context) {
        userID := getUserID(c)
        if userID == "" {
                c.JSON(http.StatusUnauthorized, gin.H{
                        "error": "用户未认证",
                })
                return
        }

        summary, err := h.service.GetUserCreditSummary(c.Request.Context(), userID)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取积分摘要失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": summary,
        })
}

// handleCreateCreditPackage 创建积分套餐（管理员）
// @Summary 创建积分套餐
// @Description 管理员创建新的积分套餐
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param package body CreditPackageRequest true "套餐信息"
// @Success 201 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/credit-packages [post]
func (h *Handler) HandleCreateCreditPackage(c *gin.Context) {
        var req CreditPackageRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "无效的请求格式",
                        "message": err.Error(),
                })
                return
        }

        // 参数验证
        if err := validateCreditPackageRequest(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": err.Error(),
                })
                return
        }

        pkg := &config.CreditPackage{
                Name:          req.Name,
                NameEN:        req.NameEN,
                Description:   req.Description,
                PriceUSDT:     req.PriceUSDT,
                Credits:       req.Credits,
                BonusCredits:  req.BonusCredits,
                IsActive:      req.IsActive,
                IsRecommended: req.IsRecommended,
                SortOrder:     req.SortOrder,
        }

        if err := h.service.CreatePackage(c.Request.Context(), pkg); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "创建套餐失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusCreated, gin.H{
                "code": 201,
                "message": "套餐创建成功",
                "data": gin.H{
                        "id": pkg.ID,
                },
        })
}

// handleUpdateCreditPackage 更新积分套餐（管理员）
// @Summary 更新积分套餐
// @Description 管理员更新现有积分套餐
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param id path string true "套餐ID"
// @Param package body CreditPackageRequest true "套餐信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 404 {object} map[string]string "套餐不存在"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/credit-packages/{id} [put]
func (h *Handler) HandleUpdateCreditPackage(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "套餐ID不能为空",
                })
                return
        }

        var req CreditPackageRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "无效的请求格式",
                        "message": err.Error(),
                })
                return
        }

        // 参数验证
        if err := validateCreditPackageRequest(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": err.Error(),
                })
                return
        }

        pkg := &config.CreditPackage{
                ID:            id,
                Name:          req.Name,
                NameEN:        req.NameEN,
                Description:   req.Description,
                PriceUSDT:     req.PriceUSDT,
                Credits:       req.Credits,
                BonusCredits:  req.BonusCredits,
                IsActive:      req.IsActive,
                IsRecommended: req.IsRecommended,
                SortOrder:     req.SortOrder,
        }

        if err := h.service.UpdatePackage(c.Request.Context(), pkg); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "更新套餐失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "套餐更新成功",
        })
}

// handleDeleteCreditPackage 删除积分套餐（管理员）
// @Summary 删除积分套餐
// @Description 管理员删除积分套餐（软删除）
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param id path string true "套餐ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/credit-packages/{id} [delete]
func (h *Handler) HandleDeleteCreditPackage(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "套餐ID不能为空",
                })
                return
        }

        if err := h.service.DeletePackage(c.Request.Context(), id); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "删除套餐失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "套餐删除成功",
        })
}

// handleAdjustUserCredits 管理员调整用户积分
// @Summary 管理员调整用户积分
// @Description 管理员手动调整用户积分余额，需要记录审计日志
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param adjustment body AdjustCreditsRequest true "调整信息"
// @Success 200 {object} map[string]interface{} "调整成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/users/{id}/credits/adjust [post]
func (h *Handler) HandleAdjustUserCredits(c *gin.Context) {
        userID := c.Param("id")
        if userID == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "用户ID不能为空",
                })
                return
        }

        adminID := getUserID(c)
        if adminID == "" {
                c.JSON(http.StatusUnauthorized, gin.H{
                        "error": "管理员未认证",
                })
                return
        }

        var req AdjustCreditsRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "无效的请求格式",
                        "message": err.Error(),
                })
                return
        }

        // 参数验证
        if req.Amount == 0 {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "调整积分数量不能为0",
                })
                return
        }

        if len(req.Reason) < 2 || len(req.Reason) > 200 {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "调整原因长度必须在2-200字符之间",
                })
                return
        }

        // 获取客户端IP
        ipAddress := c.ClientIP()

        if err := h.service.AdjustUserCredits(c.Request.Context(), adminID, userID, req.Amount, req.Reason, ipAddress); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "调整积分失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "积分调整成功",
        })
}

// handleGetUserCreditsByAdmin 管理员获取用户积分（管理员）
// @Summary 管理员获取用户积分
// @Description 管理员查看指定用户的积分信息
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {object} map[string]interface{} "积分信息"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/users/{id}/credits [get]
func (h *Handler) HandleGetUserCreditsByAdmin(c *gin.Context) {
        userID := c.Param("id")
        if userID == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "用户ID不能为空",
                })
                return
        }

        credits, err := h.service.GetUserCredits(c.Request.Context(), userID)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取用户积分失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "available_credits": credits.AvailableCredits,
                        "total_credits": credits.TotalCredits,
                        "used_credits": credits.UsedCredits,
                },
        })
}

// handleGetUserTransactionsByAdmin 管理员获取用户积分流水（管理员）
// @Summary 管理员获取用户积分流水
// @Description 管理员查看指定用户的积分交易记录
// @Tags 积分系统-管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param page query int false "页码，默认为1"
// @Param limit query int false "每页数量，默认为20，最大100"
// @Success 200 {object} map[string]interface{} "流水列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /api/v1/admin/users/{id}/credits/transactions [get]
func (h *Handler) HandleGetUserTransactionsByAdmin(c *gin.Context) {
        userID := c.Param("id")
        if userID == "" {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "用户ID不能为空",
                })
                return
        }

        // 解析分页参数
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

        // 参数验证
        if page < 1 {
                page = 1
        }
        if limit < 1 || limit > 100 {
                limit = 20
        }

        transactions, total, err := h.service.GetUserTransactions(c.Request.Context(), userID, page, limit)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "error": "获取流水失败",
                        "message": err.Error(),
                })
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "code": 200,
                "message": "success",
                "data": gin.H{
                        "transactions": transactions,
                        "total": total,
                        "page": page,
                        "limit": limit,
                        "total_pages": (total + limit - 1) / limit,
                },
        })
}

// 请求和响应结构体

// CreditPackageRequest 积分套餐请求
type CreditPackageRequest struct {
        Name          string  `json:"name" binding:"required"`
        NameEN        string  `json:"name_en"`
        Description   string  `json:"description"`
        PriceUSDT     float64 `json:"price_usdt" binding:"required,gt=0"`
        Credits       int     `json:"credits" binding:"required,gt=0"`
        BonusCredits  int     `json:"bonus_credits"`
        IsActive      bool    `json:"is_active"`
        IsRecommended bool    `json:"is_recommended"`
        SortOrder     int     `json:"sort_order"`
}

// AdjustCreditsRequest 调整积分请求
type AdjustCreditsRequest struct {
        Amount int    `json:"amount" binding:"required"`
        Reason string `json:"reason" binding:"required,min=2,max=200"`
}

// 工具函数

// validateCreditPackageRequest 验证积分套餐请求
func validateCreditPackageRequest(req *CreditPackageRequest) error {
        if req.Name == "" {
                return fmt.Errorf("套餐名称不能为空")
        }
        if req.PriceUSDT <= 0 {
                return fmt.Errorf("价格必须大于0")
        }
        if req.Credits <= 0 {
                return fmt.Errorf("积分数量必须大于0")
        }
        if req.BonusCredits < 0 {
                return fmt.Errorf("赠送积分不能为负数")
        }
        return nil
}

// getUserID 从上下文中获取用户ID
func getUserID(c *gin.Context) string {
        // 从认证中间件获取用户ID
        // 注意: server.go 的 authMiddleware 设置的是 "user_id" 键
        if userID, exists := c.Get("user_id"); exists {
                if id, ok := userID.(string); ok {
                        return id
                }
        }
        // 兼容旧的 "userID" 键（如果其他中间件使用）
        if userID, exists := c.Get("userID"); exists {
                if id, ok := userID.(string); ok {
                        return id
                }
        }
        return ""
}

// extractUserIDFromToken 从token中提取用户ID（简化实现）
func extractUserIDFromToken(token string) string {
        // TODO: 实现实际的JWT token解析
        // 这里简化处理，直接返回token作为用户ID
        if token != "" && len(token) > 10 {
                return "user_" + token[:10]
        }
        return ""
}

// authMiddleware 认证中间件
func authMiddleware() gin.HandlerFunc {
        return func(c *gin.Context) {
                // 从请求头获取token
                authHeader := c.GetHeader("Authorization")
                if authHeader == "" {
                        c.JSON(http.StatusUnauthorized, gin.H{
                                "error": "缺少认证信息",
                        })
                        c.Abort()
                        return
                }

                // 解析Bearer token
                tokenParts := strings.Split(authHeader, " ")
                if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
                        c.JSON(http.StatusUnauthorized, gin.H{
                                "error": "无效的认证格式",
                        })
                        c.Abort()
                        return
                }

                token := tokenParts[1]

                // 验证token并获取用户ID
                // TODO: 实现token验证逻辑
                // userID, err := auth.ValidateToken(token)
                // 简化处理：从token中提取用户ID
                userID := extractUserIDFromToken(token)
                if userID == "" {
                        c.JSON(http.StatusUnauthorized, gin.H{
                                "error": "认证失败",
                                "message": "无效token",
                        })
                        c.Abort()
                        return
                }

                // 将用户ID存入上下文
                c.Set("userID", userID)
                c.Next()
        }
}

// adminMiddleware 管理员权限中间件
func adminMiddleware() gin.HandlerFunc {
        return func(c *gin.Context) {
                userID := getUserID(c)
                if userID == "" {
                        c.JSON(http.StatusUnauthorized, gin.H{
                                "error": "用户未认证",
                        })
                        c.Abort()
                        return
                }

                // 检查用户是否为管理员
                // TODO: 实现管理员权限检查
                // 这里简化处理，实际应该查询数据库验证管理员身份
                isAdmin := checkAdminPermission(userID)
                if !isAdmin {
                        c.JSON(http.StatusForbidden, gin.H{
                                "error": "需要管理员权限",
                        })
                        c.Abort()
                        return
                }

                c.Next()
        }
}

// checkAdminPermission 检查管理员权限
func checkAdminPermission(userID string) bool {
        // TODO: 实现实际的管理员权限检查逻辑
        // 暂时返回true用于测试，实际应该查询数据库
        return true
}