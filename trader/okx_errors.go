package trader

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// OKX错误码映射表
var okxErrorCodes = map[string]string{
	"0":     "Success",
	"50001": "Request header OK-ACCESS-KEY cannot be blank",
	"50002": "Request header OK-ACCESS-SIGN cannot be blank",
	"50003": "Request header OK-ACCESS-TIMESTAMP cannot be blank",
	"50004": "Request header OK-ACCESS-PASSPHRASE cannot be blank",
	"50005": "Invalid OK-ACCESS-KEY",
	"50006": "Invalid OK-ACCESS-SIGN",
	"50007": "Invalid timestamp",
	"50008": "Invalid passphrase",
	"50011": "Rate limit exceeded", // 需要重试
	"50013": "Invalid IP",
	"50014": "Invalid request method",
	"50015": "Request body cannot be blank",
	"50016": "Invalid content-type",
	"50017": "Invalid request format",
	"50027": "Account blocked",
	"50028": "User blocked",
	"50029": "API key blocked",
	"50035": "Invalid instrument ID",
	"50044": "Insufficient balance",
	"50050": "Position not found",
	"50051": "Order not found",
	"50052": "Invalid order state",
	"50054": "Invalid order type",
	"50055": "Invalid order size",
	"50056": "Invalid order price",
	"50057": "Invalid order side",
	"50058": "Invalid position side",
	"50060": "Order already cancelled",
	"50061": "Too many orders", // 需要重试
	"50062": "Invalid leverage",
	"50063": "Invalid margin mode",
	"50064": "Invalid position mode",
	"50066": "Invalid symbol",
	"50067": "Invalid amount",
	"50068": "Invalid quantity",
	"58100": "Invalid position",
	"58101": "Position not found",
	"58102": "Position already closed",
	"58103": "Position side is invalid",
	"58104": "Position size is invalid",
	"58105": "Position leverage is invalid",
	"58106": "Position margin is insufficient",
	"58107": "Position margin ratio is too low",
	"58108": "Position liquidation price is invalid",
	"58109": "Position unrealized PnL is invalid",
	"58110": "Leverage too high",
	"58111": "Leverage too low",
	"58112": "Position already exists",
	"58113": "Position not exists",
	"58114": "Position not available",
	"58115": "Position not supported",
	"58200": "Cancel order failed",
	"58201": "Order already filled",
	"58202": "Order already cancelled",
	"58203": "Order not cancellable",
	"58204": "Order not found",
	"58205": "Order not supported",
	"58206": "Order size too large",
	"58207": "Order size too small",
	"58208": "Order price too high",
	"58209": "Order price too low",
	"58210": "Order not in valid range",
	"58211": "Order not in valid state",
	"58212": "Order type not supported",
	"58213": "Order side not supported",
	"58214": "Order time not supported",
	"58215": "Order quantity not supported",
	"58216": "Order not in valid time",
	"58217": "Order not in valid date",
	"58218": "Order not in valid price",
	"58219": "Order not in valid size",
	"58220": "Order not in valid amount",
	"58221": "Order not in valid quantity",
	"58222": "Order not in valid leverage",
	"58223": "Order not in valid margin",
	"58224": "Order not in valid mode",
	"58225": "Order not in valid type",
	"58226": "Order not in valid side",
	"58227": "Order not in valid state",
	"58228": "Order not in valid status",
	"58229": "Order not in valid action",
	"58230": "Order not in valid operation",
	"50100": "Invalid API key",
	"51010": "Insufficient balance",
}

// GetErrorMessage 获取错误信息
func GetErrorMessage(code string) string {
	if msg, exists := okxErrorCodes[code]; exists {
		return msg
	}
	return "Unknown error: " + code
}

// IsRetryableError 判断错误是否应该重试
func IsRetryableError(code string) bool {
	retryableCodes := []string{
		"50011", // Rate limit exceeded
		"50061", // Too many orders
		"58200", // Cancel order failed
	}

	for _, retryable := range retryableCodes {
		if code == retryable {
			return true
		}
	}
	return false
}

// IsAuthenticationError 判断是否为认证错误
func IsAuthenticationError(code string) bool {
	authErrorCodes := []string{
		"50001", // Missing API key
		"50002", // Missing signature
		"50003", // Missing timestamp
		"50004", // Missing passphrase
		"50005", // Invalid API key
		"50006", // Invalid signature
		"50007", // Invalid timestamp
		"50008", // Invalid passphrase
		"50013", // Invalid IP
		"50029", // API key blocked
	}

	for _, authError := range authErrorCodes {
		if code == authError {
			return true
		}
	}
	return false
}

// OKXAPIError OKX API错误类型
type OKXAPIError struct {
	Code    string
	Message string
	Data    interface{}
}

// Error 实现error接口
func (e *OKXAPIError) Error() string {
	return fmt.Sprintf("OKX API Error [%s]: %s", e.Code, e.Message)
}

// NewOKXAPIError 创建OKX API错误
func NewOKXAPIError(code, message string, data interface{}) *OKXAPIError {
	return &OKXAPIError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// RetryStrategy 重试策略
type RetryStrategy struct {
	MaxRetries     int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	BackoffFactor  float64
}

// DefaultRetryStrategy 默认重试策略
var DefaultRetryStrategy = RetryStrategy{
	MaxRetries:    3,
	InitialDelay:  1 * time.Second,
	MaxDelay:      30 * time.Second,
	BackoffFactor: 2.0,
}

// CalculateDelay 计算重试延迟
func (rs *RetryStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := float64(rs.InitialDelay) * math.Pow(rs.BackoffFactor, float64(attempt-1))

	if delay > float64(rs.MaxDelay) {
		delay = float64(rs.MaxDelay)
	}

	return time.Duration(delay)
}

// getOKXError 获取OKX错误信息
func getOKXError(code string) string {
	if msg, exists := okxErrorCodes[code]; exists {
		return msg
	}
	return fmt.Sprintf("Unknown error: %s", code)
}

// isRetryableOKXError 判断错误是否可重试
func isRetryableOKXError(code string) bool {
	retryableCodes := []string{"50011", "50061"}
	for _, retryableCode := range retryableCodes {
		if code == retryableCode {
			return true
		}
	}
	return false
}

// ShouldRetry 判断是否应该重试
func (rs *RetryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= rs.MaxRetries {
		return false
	}

	if err == nil {
		return false
	}

	// 检查是否为OKX API错误
	var okxErr *OKXAPIError
	if errors.As(err, &okxErr) {
		return IsRetryableError(okxErr.Code)
	}

	// 检查错误消息
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"rate limit",
		"too many",
		"timeout",
		"connection refused",
		"temporary failure",
		"try again",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// ValidateCredentials 验证OKX凭证格式
func ValidateCredentials(apiKey, secretKey, passphrase string) error {
	if apiKey == "" {
		return fmt.Errorf("API密钥不能为空")
	}
	if len(apiKey) < 10 {
		return fmt.Errorf("API密钥长度不能少于10个字符")
	}
	if len(apiKey) > 50 {
		return fmt.Errorf("API密钥长度不能超过50个字符")
	}

	if secretKey == "" {
		return fmt.Errorf("Secret密钥不能为空")
	}
	if len(secretKey) < 20 {
		return fmt.Errorf("Secret密钥长度不能少于20个字符")
	}
	if len(secretKey) > 100 {
		return fmt.Errorf("Secret密钥长度不能超过100个字符")
	}

	if passphrase == "" {
		return fmt.Errorf("Passphrase不能为空")
	}
	if len(passphrase) < 6 {
		return fmt.Errorf("Passphrase长度不能少于6个字符")
	}
	if len(passphrase) > 50 {
		return fmt.Errorf("Passphrase长度不能超过50个字符")
	}

	return nil
}

// RateLimiter OKX速率限制器
type RateLimiter struct {
	rate       int           // 每秒允许的请求数
	burst      int           // 突发请求数
	tokens     chan struct{}
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     make(chan struct{}, burst),
		lastRefill: time.Now(),
	}
}

// Wait 等待获取令牌
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 没有可用令牌，等待
		select {
		case <-rl.tokens:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return fmt.Errorf("rate limit exceeded")
		}
	}
}

// Refill 补充令牌
func (rl *RateLimiter) Refill() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))

	if tokensToAdd > 0 {
		for i := 0; i < tokensToAdd && i < rl.burst; i++ {
			select {
			case rl.tokens <- struct{}{}:
			default:
				// 通道已满
				return
			}
		}
		rl.lastRefill = now
	}
}

// OKXRateLimits OKX默认速率限制
var OKXRateLimits = struct {
	PublicAPI   RateLimiter
	PrivateAPI  RateLimiter
	TradingAPI  RateLimiter
}{
	PublicAPI:  RateLimiter{rate: 10, burst: 20},   // 10 req/s, burst 20
	PrivateAPI: RateLimiter{rate: 5, burst: 10},    // 5 req/s, burst 10
	TradingAPI: RateLimiter{rate: 2, burst: 5},     // 2 req/s, burst 5 (保守设置)
}

// OKXRateLimitRequestsPerSecond OKX速率限制每秒请求数
const OKXRateLimitRequestsPerSecond = 5

// OKXRateLimitBurst OKX速率限制突发请求数
const OKXRateLimitBurst = 10

// GetRateLimiterForEndpoint 获取指定端点的速率限制器
func GetRateLimiterForEndpoint(endpoint string) *RateLimiter {
	switch {
	case strings.Contains(endpoint, "/market/"):
		return &OKXRateLimits.PublicAPI
	case strings.Contains(endpoint, "/account/"):
		return &OKXRateLimits.PrivateAPI
	case strings.Contains(endpoint, "/trade/"):
		return &OKXRateLimits.TradingAPI
	default:
		return &OKXRateLimits.PrivateAPI
	}
}

// TimestampValidator 时间戳验证器
type TimestampValidator struct {
	MaxDrift time.Duration
}

// NewTimestampValidator 创建时间戳验证器
func NewTimestampValidator(maxDrift time.Duration) *TimestampValidator {
	return &TimestampValidator{
		MaxDrift: maxDrift,
	}
}

// Validate 验证时间戳是否在允许范围内
func (tv *TimestampValidator) Validate(timestamp string) error {
	t, err := time.Parse("2006-01-02T15:04:05.000Z", timestamp)
	if err != nil {
		return fmt.Errorf("无效的时间戳格式: %w", err)
	}

	now := time.Now().UTC()
	drift := now.Sub(t)

	if drift < 0 {
		drift = -drift
	}

	if drift > tv.MaxDrift {
		return fmt.Errorf("时间戳漂移过大: %v > %v", drift, tv.MaxDrift)
	}

	return nil
}

// DefaultTimestampValidator 默认时间戳验证器（±30秒）
var DefaultTimestampValidator = NewTimestampValidator(30 * time.Second)

// SecurityValidator 安全验证器
type SecurityValidator struct {
	TimestampValidator *TimestampValidator
	RateLimiter        *RateLimiter
}

// NewSecurityValidator 创建安全验证器
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		TimestampValidator: DefaultTimestampValidator,
		RateLimiter:        &OKXRateLimits.PrivateAPI,
	}
}

// ValidateRequest 验证请求的安全性
func (sv *SecurityValidator) ValidateRequest(ctx context.Context, timestamp string) error {
	// 验证时间戳
	if err := sv.TimestampValidator.Validate(timestamp); err != nil {
		return fmt.Errorf("时间戳验证失败: %w", err)
	}

	// 检查速率限制
	if err := sv.RateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("速率限制检查失败: %w", err)
	}

	return nil
}

// LogSecurityEvent 记录安全事件
func LogSecurityEvent(event, details string) {
	log.Printf("🛡️ 安全事件: %s - %s", event, details)
}

// SanitizeForLogging 清理敏感信息用于日志
func SanitizeForLogging(data string) string {
	if len(data) <= 8 {
		return "****"
	}
	return data[:4] + "****" + data[len(data)-4:]
}

// validateOKXAPIKey 验证API密钥
func validateOKXAPIKey(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("API key cannot be empty")
	}
	if len(apiKey) < 10 {
		return errors.New("API key too short")
	}
	// 检查是否包含无效字符
	if strings.ContainsAny(apiKey, "@#$%^&*()+=[]{}|;:'\",<>?/") {
		return errors.New("API key contains invalid characters")
	}
	return nil
}

// validateOKXSecretKey 验证密钥
func validateOKXSecretKey(secretKey string) error {
	if strings.TrimSpace(secretKey) == "" {
		return errors.New("Secret key cannot be empty")
	}
	if len(secretKey) < 10 {
		return errors.New("Secret key too short")
	}
	return nil
}

// validateOKXPassphrase 验证密码短语
func validateOKXPassphrase(passphrase string) error {
	if strings.TrimSpace(passphrase) == "" {
		return errors.New("Passphrase cannot be empty")
	}
	if len(passphrase) < 6 {
		return errors.New("Passphrase too short")
	}
	return nil
}

// validateOKXSymbol 验证交易对符号
func validateOKXSymbol(symbol string) error {
	if strings.TrimSpace(symbol) == "" {
		return errors.New("Symbol cannot be empty")
	}
	// 基本格式验证：字母-字母（如BTC-USDT）
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return errors.New("Invalid symbol format")
	}
	// 检查是否只包含字母
	for _, part := range parts {
		if !isValidCurrencyCode(part) {
			return errors.New("Invalid symbol format")
		}
	}
	return nil
}

// isValidCurrencyCode 检查货币代码是否有效
func isValidCurrencyCode(code string) bool {
	if code == "" || len(code) > 10 {
		return false
	}
	for _, char := range code {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')) {
			return false
		}
	}
	return true
}

// validateOKXQuantity 验证数量
func validateOKXQuantity(quantity float64) error {
	if quantity <= 0 {
		return errors.New("Quantity must be greater than 0")
	}
	if quantity < 0.0001 {
		return errors.New("Quantity too small")
	}
	if quantity > 10000 {
		return errors.New("Quantity too large")
	}
	return nil
}

// validateOKXLeverage 验证杠杆倍数
func validateOKXLeverage(leverage int) error {
	if leverage <= 0 {
		return errors.New("Leverage must be greater than 0")
	}
	if leverage > 125 {
		return errors.New("Leverage too high")
	}
	return nil
}

// validateOKXPrice 验证价格
func validateOKXPrice(price float64) error {
	if price <= 0 {
		return errors.New("Price must be greater than 0")
	}
	if price > 1000000000 {
		return errors.New("Price too high")
	}
	return nil
}

// validateOKXCredentials 验证所有凭据
func validateOKXCredentials(apiKey, secretKey, passphrase string) error {
	if err := validateOKXAPIKey(apiKey); err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}
	if err := validateOKXSecretKey(secretKey); err != nil {
		return fmt.Errorf("invalid secret key: %w", err)
	}
	if err := validateOKXPassphrase(passphrase); err != nil {
		return fmt.Errorf("invalid passphrase: %w", err)
	}
	return nil
}

// validateOKXParameters 验证所有参数
func validateOKXParameters(symbol string, quantity float64, leverage int, price float64) error {
	if err := validateOKXSymbol(symbol); err != nil {
		return fmt.Errorf("invalid symbol: %w", err)
	}
	if err := validateOKXQuantity(quantity); err != nil {
		return fmt.Errorf("invalid quantity: %w", err)
	}
	if err := validateOKXLeverage(leverage); err != nil {
		return fmt.Errorf("invalid leverage: %w", err)
	}
	if err := validateOKXPrice(price); err != nil {
		return fmt.Errorf("invalid price: %w", err)
	}
	return nil
}

// sanitizeAPIKey 清理API密钥用于显示
func sanitizeAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

// sanitizeError 清理错误消息中的敏感信息
func sanitizeError(error string) string {
	// 清理API密钥
	error = strings.ReplaceAll(error, "1234567890abcdef", "1234****cdef")
	error = strings.ReplaceAll(error, "abcdef1234567890", "abcd****7890")
	// 清理密码短语
	error = strings.ReplaceAll(error, "mysecretpassphrase", "myse****rase")
	return error
}

// ValidateSymbol 验证交易对格式
func ValidateSymbol(symbol string) error {
	if symbol == "" {
		return fmt.Errorf("交易对不能为空")
	}

	// OKX标准格式: BASE-QUOTE-SWAP (永续合约)
	parts := strings.Split(symbol, "-")
	if len(parts) != 3 {
		return fmt.Errorf("交易对格式无效，应为 BASE-QUOTE-SWAP 格式")
	}

	base := parts[0]
	quote := parts[1]
	suffix := parts[2]

	if base == "" || quote == "" {
		return fmt.Errorf("交易对的基础货币或报价货币不能为空")
	}

	if suffix != "SWAP" {
		return fmt.Errorf("只支持永续合约 (SWAP)")
	}

	return nil
}

// ValidateQuantity 验证数量
func ValidateQuantity(quantity float64) error {
	if quantity <= 0 {
		return fmt.Errorf("数量必须大于0")
	}

	if quantity > 1000000 {
		return fmt.Errorf("数量不能超过1000000")
	}

	return nil
}

// ValidatePrice 验证价格
func ValidatePrice(price float64) error {
	if price < 0 {
		return fmt.Errorf("价格不能为负数")
	}

	if price > 10000000 {
		return fmt.Errorf("价格不能超过10000000")
	}

	return nil
}

// ValidateLeverage 验证杠杆
func ValidateLeverage(leverage int) error {
	if leverage < 1 {
		return fmt.Errorf("杠杆不能小于1")
	}

	if leverage > 125 {
		return fmt.Errorf("杠杆不能超过125")
	}

	return nil
}

// StandardizeError 标准化错误信息
func StandardizeError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	// 标准化网络错误
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("网络连接失败，请检查网络设置")
	}

	if strings.Contains(errStr, "timeout") {
		return fmt.Errorf("请求超时，请稍后重试")
	}

	if strings.Contains(errStr, "rate limit") {
		return fmt.Errorf("请求频率过高，请稍后重试")
	}

	// 标准化认证错误
	if strings.Contains(errStr, "invalid api key") {
		return fmt.Errorf("API密钥无效，请检查配置")
	}

	if strings.Contains(errStr, "invalid signature") {
		return fmt.Errorf("签名验证失败，请检查密钥配置")
	}

	// 标准化交易错误
	if strings.Contains(errStr, "insufficient balance") {
		return fmt.Errorf("账户余额不足")
	}

	if strings.Contains(errStr, "position not found") {
		return fmt.Errorf("未找到指定持仓")
	}

	if strings.Contains(errStr, "order not found") {
		return fmt.Errorf("未找到指定订单")
	}

	// 保持原始错误，如果无法标准化
	return err
}