package handlers

import (
        "database/sql"
        "log"
        "net/http"
        "os"
        "time"

        "github.com/gin-gonic/gin"
        "github.com/google/uuid"

        "nofx/auth"
        "nofx/config"
)

// HandleRegister 处理用户注册请求
func (h *BaseHandler) HandleRegister(c *gin.Context) {
        var req struct {
                Email    string `json:"email" binding:"required,email"`
                Password string `json:"password" binding:"required,min=8"`
                BetaCode string `json:"beta_code"`
        }

        // 验证请求数据
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{
                        "success": false,
                        "error":   "请求数据格式错误",
                        "details": "请确保邮箱格式正确，密码长度不少于8位",
                })
                return
        }

        // 验证密码强度
        if len(req.Password) < 8 {
                c.JSON(http.StatusBadRequest, gin.H{
                        "success": false,
                        "error":   "密码强度不够",
                        "details": "密码必须至少包含8个字符",
                })
                return
        }

        // 检查是否开启了内测模式
        betaModeStr, _ := h.Database.GetSystemConfig("beta_mode")
        if betaModeStr == "true" {
                // 内测模式下必须提供有效的内测码
                if req.BetaCode == "" {
                        c.JSON(http.StatusBadRequest, gin.H{
                                "success": false,
                                "error":   "内测码不能为空",
                                "details": "当前为内测期间，注册需要提供有效的内测码",
                        })
                        return
                }

                // 验证内测码
                isValid, err := h.Database.ValidateBetaCode(req.BetaCode)
                if err != nil {
                        c.JSON(http.StatusInternalServerError, gin.H{
                                "success": false,
                                "error":   "内测码验证失败",
                                "details": "服务器内部错误，请稍后重试",
                        })
                        return
                }
                if !isValid {
                        c.JSON(http.StatusBadRequest, gin.H{
                                "success": false,
                                "error":   "内测码无效",
                                "details": "内测码无效或已被使用，请检查后重试",
                        })
                        return
                }
        }

        // 检查邮箱是否已存在
        _, err := h.Database.GetUserByEmail(req.Email)
        if err == nil {
                c.JSON(http.StatusConflict, gin.H{
                        "success": false,
                        "error":   "邮箱已被注册",
                        "details": "该邮箱地址已经注册，请使用其他邮箱或尝试登录",
                })
                return
        }
        if err != sql.ErrNoRows {
                // 数据库查询失败，不是用户不存在的错误
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "error":   "系统错误",
                        "details": "服务器内部错误，请稍后重试",
                })
                return
        }

        // 生成密码哈希
        passwordHash, err := auth.HashPassword(req.Password)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "error":   "密码处理失败",
                        "details": "服务器内部错误，请稍后重试",
                })
                return
        }

        // 创建用户（直接激活，无需OTP验证）
        userID := uuid.New().String()
        now := time.Now()
        user := &config.User{
                ID:             userID,
                Email:          req.Email,
                PasswordHash:   passwordHash,
                OTPSecret:      "",   // 移除OTP密钥
                OTPVerified:    true, // 直接标记为已验证
                IsActive:       true, // 账户激活状态
                IsAdmin:        false, // 非管理员
                BetaCode:       req.BetaCode, // 关联内测码
                FailedAttempts: 0,    // 失败尝试次数
                CreatedAt:      now,  // 创建时间
                UpdatedAt:      now,  // 更新时间
        }

        err = h.Database.CreateUser(user)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "error":   "创建用户失败",
                        "details": "服务器内部错误，请稍后重试",
                })
                return
        }

        // 如果是内测模式，标记内测码为已使用
        betaModeStr2, _ := h.Database.GetSystemConfig("beta_mode")
        if betaModeStr2 == "true" && req.BetaCode != "" {
                err := h.Database.UseBetaCode(req.BetaCode, req.Email)
                if err != nil {
                        log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
                        // 这里不返回错误，因为用户已经创建成功
                } else {
                        log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
                }
        }

        // 生成JWT令牌
        token, err := auth.GenerateJWT(userID, req.Email)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                        "success": false,
                        "error":   "令牌生成失败",
                        "details": "服务器内部错误，请稍后重试",
                })
                return
        }

        // 返回成功信息
        c.JSON(http.StatusOK, gin.H{
                "success": true,
                "message": "注册成功，欢迎加入Monnaire Trading Agent OS！",
                "token":   token,
                "user": gin.H{
                        "id":    userID,
                        "email": req.Email,
                },
        })
}

// HandleCompleteRegistration 完成注册（验证OTP）
func (h *BaseHandler) HandleCompleteRegistration(c *gin.Context) {
        var req struct {
                UserID  string `json:"user_id" binding:"required"`
                OTPCode string `json:"otp_code" binding:"required"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        // 获取用户信息
        user, err := h.Database.GetUserByID(req.UserID)
        if err != nil {
                c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
                return
        }

        // 验证OTP
        if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
                return
        }

        // 更新用户OTP验证状态
        err = h.Database.UpdateUserOTPVerified(req.UserID, true)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
                return
        }

        // 生成JWT token
        token, err := auth.GenerateJWT(user.ID, user.Email)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "token":   token,
                "user_id": user.ID,
                "email":   user.Email,
                "message": "注册完成",
        })
}

// HandleLogin 处理用户登录请求
func (h *BaseHandler) HandleLogin(c *gin.Context) {
        var req struct {
                Email    string `json:"email" binding:"required,email"`
                Password string `json:"password" binding:"required"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
                log.Printf("❌ [Login] 请求解析失败: %v", err)
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        log.Printf("🔐 [Login] 登录请求: email=%s", req.Email)

        // 获取用户信息
        user, err := h.Database.GetUserByEmail(req.Email)
        if err != nil {
                log.Printf("❌ [Login] 获取用户失败: email=%s, error=%v", req.Email, err)
                c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
                return
        }

        log.Printf("✓ [Login] 找到用户: id=%s, email=%s, hash_len=%d", user.ID, user.Email, len(user.PasswordHash))

        // 验证密码
        if !auth.CheckPassword(req.Password, user.PasswordHash) {
                log.Printf("❌ [Login] 密码验证失败: email=%s", req.Email)
                c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
                return
        }

        log.Printf("✓ [Login] 密码验证成功: email=%s", req.Email)

        // 检查是否开启内测模式
        betaModeStr, _ := h.Database.GetSystemConfig("beta_mode")
        if betaModeStr == "true" {
                // 内测模式下，验证用户是否有有效的内测码
                userBetaCode, err := h.Database.GetUserBetaCode(user.ID)
                if err != nil {
                        log.Printf("⚠️ 获取用户内测码失败: %v", err)
                        c.JSON(http.StatusInternalServerError, gin.H{"error": "验证失败，请稍后重试"})
                        return
                }

                if userBetaCode == "" {
                        c.JSON(http.StatusUnauthorized, gin.H{"error": "内测码无效，请联系管理员"})
                        return
                }

                // 验证内测码是否仍然有效
                isValid, err := h.Database.ValidateBetaCode(userBetaCode)
                if err != nil {
                        log.Printf("⚠️ 验证内测码失败: %v", err)
                        c.JSON(http.StatusInternalServerError, gin.H{"error": "验证失败，请稍后重试"})
                        return
                }

                if !isValid {
                        c.JSON(http.StatusUnauthorized, gin.H{"error": "内测码无效，请联系管理员"})
                        return
                }

                log.Printf("✓ 用户 %s 登录成功（内测码: %s）", user.Email, userBetaCode)
        }

        // 生成JWT token
        token, err := auth.GenerateJWT(user.ID, user.Email)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
                return
        }

        // 返回成功信息
        c.JSON(http.StatusOK, gin.H{
                "token":   token,
                "user_id": user.ID,
                "email":   user.Email,
                "message": "登录成功",
        })
}

// HandleVerifyOTP 验证OTP并完成登录
func (h *BaseHandler) HandleVerifyOTP(c *gin.Context) {
        var req struct {
                UserID  string `json:"user_id" binding:"required"`
                OTPCode string `json:"otp_code" binding:"required"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        // 获取用户信息
        user, err := h.Database.GetUserByID(req.UserID)
        if err != nil {
                c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
                return
        }

        // 验证OTP
        if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
                return
        }

        // 生成JWT token
        token, err := auth.GenerateJWT(user.ID, user.Email)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
                return
        }

        c.JSON(http.StatusOK, gin.H{
                "token":   token,
                "user_id": user.ID,
                "email":   user.Email,
                "message": "登录成功",
        })
}

// HandleRequestPasswordReset 处理密码重置请求
func (h *BaseHandler) HandleRequestPasswordReset(c *gin.Context) {
        var req struct {
                Email string `json:"email" binding:"required,email"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        // 检查用户是否存在
        user, err := h.Database.GetUserByEmail(req.Email)
        if err != nil {
                // 即使用户不存在，也返回成功，防止邮箱枚举攻击
                c.JSON(http.StatusOK, gin.H{
                        "message": "如果该邮箱已注册，您将收到密码重置邮件",
                })
                return
        }

        // 检查IP频率限制
        ipAddress := auth.ExtractIPFromRequest(map[string]string{
                "X-Forwarded-For": c.GetHeader("X-Forwarded-For"),
                "X-Real-IP":       c.GetHeader("X-Real-IP"),
        })

        failedAttempts, err := h.Database.GetLoginAttemptsByIP(ipAddress)
        if err != nil {
                log.Printf("获取IP登录尝试次数失败: %v", err)
        }

        // 检查邮箱频率限制
        emailAttempts, err := h.Database.GetLoginAttemptsByEmail(req.Email)
        if err != nil {
                log.Printf("获取邮箱登录尝试次数失败: %v", err)
        }

        // 频率限制：每IP每小时最多3次，每邮箱每小时最多3次
        if failedAttempts >= 3 || emailAttempts >= 3 {
                c.JSON(http.StatusTooManyRequests, gin.H{
                        "error": "请求过于频繁，请稍后再试",
                })
                return
        }

        // 生成密码重置令牌
        token, err := auth.GeneratePasswordResetToken()
        if err != nil {
                log.Printf("生成密码重置令牌失败: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "生成重置令牌失败"})
                return
        }

        tokenHash := auth.HashPasswordResetToken(token)
        expiresAt := time.Now().Add(1 * time.Hour)

        // 存储令牌
        err = h.Database.CreatePasswordResetToken(user.ID, token, tokenHash, expiresAt)
        if err != nil {
                log.Printf("存储密码重置令牌失败: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "创建重置令牌失败"})
                return
        }

        // 获取前端URL（从环境变量或使用默认值）
        frontendURL := os.Getenv("FRONTEND_URL")
        if frontendURL == "" {
                frontendURL = "https://web-pink-omega-40.vercel.app" // 默认前端URL
        }

        // 发送密码重置邮件
        err = h.EmailClient.SendPasswordResetEmail(req.Email, token, frontendURL)
        if err != nil {
                log.Printf("❌ 发送密码重置邮件失败: %v", err)
                // 即使邮件发送失败，也返回成功消息（防止邮箱枚举）
                // 但记录错误日志供管理员查看
        } else {
                log.Printf("✅ 密码重置邮件已发送 - 收件人: %s", req.Email)
        }

        c.JSON(http.StatusOK, gin.H{
                "message": "如果该邮箱已注册，您将收到密码重置邮件",
        })
}

// HandleResetPassword 处理密码重置确认
func (h *BaseHandler) HandleResetPassword(c *gin.Context) {
        var req struct {
                Token    string `json:"token" binding:"required"`
                Password string `json:"password" binding:"required,min=8"`
                OTPCode  string `json:"otp_code" binding:"required"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        // 验证令牌
        tokenHash := auth.HashPasswordResetToken(req.Token)
        userID, err := h.Database.ValidatePasswordResetToken(tokenHash)
        if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "重置链接无效或已过期"})
                return
        }

        // 获取用户信息
        user, err := h.Database.GetUserByID(*userID)
        if err != nil {
                c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
                return
        }

        // 验证OTP
        if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
                return
        }

        // 生成新密码哈希
        newPasswordHash, err := auth.HashPassword(req.Password)
        if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        // 更新密码
        err = h.Database.UpdateUserPassword(user.ID, newPasswordHash)
        if err != nil {
                log.Printf("更新用户密码失败: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
                return
        }

        // 标记令牌为已使用
        err = h.Database.MarkPasswordResetTokenAsUsed(tokenHash)
        if err != nil {
                log.Printf("标记令牌为已使用失败: %v", err)
        }

        // 使用户的所有其他令牌失效
        err = h.Database.InvalidateAllPasswordResetTokens(user.ID)
        if err != nil {
                log.Printf("使其他令牌失效失败: %v", err)
        }

        // 重置失败尝试次数
        err = h.Database.ResetUserFailedAttempts(user.ID)
        if err != nil {
                log.Printf("重置用户失败尝试次数失败: %v", err)
        }

        c.JSON(http.StatusOK, gin.H{
                "message": "密码重置成功，请使用新密码登录",
        })
}
