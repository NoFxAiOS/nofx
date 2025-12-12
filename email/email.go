package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// ResendClient Resend邮件客户端
type ResendClient struct {
	apiKey     string
	apiURL     string
	fromEmail  string
	fromName   string
	httpClient *http.Client
}

// EmailRequest 邮件请求
type EmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Html    string   `json:"html"`
	Text    string   `json:"text,omitempty"`
}

// EmailResponse Resend API响应
type EmailResponse struct {
	ID    string `json:"id"`
	Error string `json:"error,omitempty"`
}

// NewResendClient 创建Resend客户端
func NewResendClient() *ResendClient {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("⚠️  RESEND_API_KEY未设置，邮件发送功能将不可用")
	}

	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "noreply@yourdomain.com" // 默认发件人
		log.Printf("⚠️  RESEND_FROM_EMAIL未设置，使用默认值: %s", fromEmail)
	}

	fromName := os.Getenv("RESEND_FROM_NAME")
	if fromName == "" {
		fromName = "Monnaire Trading Agent OS"
	}

	return &ResendClient{
		apiKey:    apiKey,
		apiURL:    "https://api.resend.com/emails",
		fromEmail: fromEmail,
		fromName:  fromName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HasAPIKey 检查是否配置了API Key
func (c *ResendClient) HasAPIKey() bool {
	return c.apiKey != ""
}

// GetFromEmail 获取发件人邮箱
func (c *ResendClient) GetFromEmail() string {
	return c.fromEmail
}

// SendEmail 发送邮件
func (c *ResendClient) SendEmail(to, subject, htmlContent, textContent string) error {
	if c.apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY未配置")
	}

	// 构建发件人地址（带名称）
	from := fmt.Sprintf("%s <%s>", c.fromName, c.fromEmail)

	// 构建请求
	emailReq := EmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlContent,
		Text:    textContent,
	}

	jsonData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("序列化邮件请求失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送邮件请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var emailResp EmailResponse
	if err := json.Unmarshal(body, &emailResp); err != nil {
		return fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("邮件发送失败 (状态码: %d): %s", resp.StatusCode, emailResp.Error)
	}

	log.Printf("✅ 邮件发送成功 - 收件人: %s, 邮件ID: %s", to, emailResp.ID)
	return nil
}

// SendPasswordResetEmail 发送密码重置邮件
func (c *ResendClient) SendPasswordResetEmail(to, resetToken, frontendURL string) error {
	// 构建重置链接
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken)

	// 生成HTML内容
	htmlContent, err := generatePasswordResetHTML(resetLink)
	if err != nil {
		return fmt.Errorf("生成邮件HTML失败: %w", err)
	}

	// 生成纯文本内容（作为备用）
	textContent := fmt.Sprintf(`
密码重置请求

您好，

我们收到了您的密码重置请求。请点击以下链接重置您的密码：

%s

此链接将在1小时后过期。

如果您没有请求重置密码，请忽略此邮件。

---
Monnaire Trading Agent OS
`, resetLink)

	// 发送邮件
	subject := "密码重置 - Monnaire Trading Agent OS"
	return c.SendEmail(to, subject, htmlContent, textContent)
}

// generatePasswordResetHTML 生成密码重置邮件的HTML内容
func generatePasswordResetHTML(resetLink string) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>密码重置</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo {
            font-size: 24px;
            font-weight: bold;
            color: #4F46E5;
            margin-bottom: 10px;
        }
        h1 {
            color: #1F2937;
            font-size: 24px;
            margin-bottom: 20px;
        }
        p {
            color: #4B5563;
            margin-bottom: 20px;
        }
        .button {
            display: inline-block;
            padding: 14px 32px;
            background-color: #4F46E5;
            color: #ffffff !important;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            margin: 20px 0;
            transition: background-color 0.3s;
        }
        .button:hover {
            background-color: #4338CA;
        }
        .link-box {
            background-color: #F3F4F6;
            padding: 15px;
            border-radius: 6px;
            margin: 20px 0;
            word-break: break-all;
            font-size: 12px;
            color: #6B7280;
        }
        .warning {
            background-color: #FEF3C7;
            border-left: 4px solid #F59E0B;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .footer {
            text-align: center;
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #E5E7EB;
            color: #9CA3AF;
            font-size: 14px;
        }
        .security-tips {
            background-color: #EFF6FF;
            border-left: 4px solid #3B82F6;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">🤖 Monnaire Trading Agent OS</div>
        </div>
        
        <h1>密码重置请求</h1>
        
        <p>您好，</p>
        
        <p>我们收到了您的密码重置请求。请点击下方按钮重置您的密码：</p>
        
        <div style="text-align: center;">
            <a href="{{.ResetLink}}" class="button">重置密码</a>
        </div>
        
        <p>或者复制以下链接到浏览器中打开：</p>
        
        <div class="link-box">
            {{.ResetLink}}
        </div>
        
        <div class="warning">
            <strong>⚠️ 重要提示：</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>此链接将在 <strong>1小时</strong> 后过期</li>
                <li>链接只能使用 <strong>一次</strong></li>
                <li>重置密码时需要输入您的 <strong>OTP验证码</strong></li>
            </ul>
        </div>
        
        <div class="security-tips">
            <strong>🔒 安全提示：</strong>
            <ul style="margin: 10px 0; padding-left: 20px;">
                <li>如果您没有请求重置密码，请忽略此邮件</li>
                <li>请勿将此链接分享给任何人</li>
                <li>我们永远不会通过邮件询问您的密码</li>
            </ul>
        </div>
        
        <div class="footer">
            <p>此邮件由系统自动发送，请勿直接回复。</p>
            <p>&copy; 2025 Monnaire Trading Agent OS. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

	// 解析模板
	t, err := template.New("passwordReset").Parse(tmpl)
	if err != nil {
		return "", err
	}

	// 执行模板
	var buf bytes.Buffer
	data := struct {
		ResetLink string
	}{
		ResetLink: resetLink,
	}

	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SendWelcomeEmail 发送欢迎邮件（可选功能）
func (c *ResendClient) SendWelcomeEmail(to, userName string) error {
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4F46E5; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .button { display: inline-block; padding: 10px 20px; background-color: #4F46E5; color: white; text-decoration: none; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>欢迎加入 Monnaire Trading Agent OS！</h1>
        </div>
        <div class="content">
            <p>您好 %s，</p>
            <p>感谢您注册 Monnaire Trading Agent OS！</p>
            <p>您现在可以开始创建和管理您的AI交易员了。</p>
            <p>祝您交易顺利！</p>
        </div>
    </div>
</body>
</html>
`, userName)

	textContent := fmt.Sprintf("欢迎加入 Monnaire Trading Agent OS！\n\n您好 %s，\n\n感谢您的注册！", userName)

	subject := "欢迎加入 Monnaire Trading Agent OS"
	return c.SendEmail(to, subject, htmlContent, textContent)
}

// SendEmailWithRetry 带重试机制的邮件发送
func (c *ResendClient) SendEmailWithRetry(to, subject, htmlContent, textContent string) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := c.SendEmail(to, subject, htmlContent, textContent)
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("⚠️  [EMAIL_RETRY] 邮件发送失败 (尝试 %d/%d)", attempt, maxRetries)
		log.Printf("   收件人: %s", to)
		log.Printf("   错误: %v", err)

		if attempt < maxRetries {
			// 指数退避: 1s, 2s, 4s
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("   等待 %v 后重试...", delay)
			time.Sleep(delay)
		}
	}

	log.Printf("🔴 [EMAIL_FAILED] 邮件发送失败，已重试%d次", maxRetries)
	return fmt.Errorf("邮件发送失败（已重试%d次）: %w", maxRetries, lastErr)
}

// SendPasswordResetEmailWithRetry 带重试的密码重置邮件发送
func (c *ResendClient) SendPasswordResetEmailWithRetry(to, resetToken, frontendURL string) error {
	// 构建重置链接
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken)

	// 生成HTML内容
	htmlContent, err := generatePasswordResetHTML(resetLink)
	if err != nil {
		return fmt.Errorf("生成邮件HTML失败: %w", err)
	}

	// 生成纯文本内容（作为备用）
	textContent := fmt.Sprintf(`
密码重置请求

您好，

我们收到了您的密码重置请求。请点击以下链接重置您的密码：

%s

此链接将在1小时后过期。

如果您没有请求重置密码，请忽略此邮件。

---
Monnaire Trading Agent OS
`, resetLink)

	// 发送邮件（带重试）
	subject := "密码重置 - Monnaire Trading Agent OS"
	return c.SendEmailWithRetry(to, subject, htmlContent, textContent)
}
