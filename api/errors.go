package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"nofx/logger"
)

// SafeError returns a safe error message without exposing internal details
// It logs the actual error for debugging but returns a generic message to the client
func SafeError(c *gin.Context, statusCode int, publicMsg string, internalErr error) {
	// Log the actual error internally
	if internalErr != nil {
		logger.Errorf("[API Error] %s: %v", publicMsg, internalErr)
	}

	c.JSON(statusCode, gin.H{"error": publicMsg})
}

// SafeInternalError logs internal error and returns a generic message
func SafeInternalError(c *gin.Context, operation string, err error) {
	logger.Errorf("[Internal Error] %s: %v", operation, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": operation + " failed"})
}

// SafeBadRequest returns a safe bad request error
// For validation errors, we can be more specific since they're about user input
func SafeBadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}

// SafeNotFound returns a generic not found error
func SafeNotFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, gin.H{"error": resource + " not found"})
}

// SafeUnauthorized returns unauthorized error
func SafeUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}

// SafeForbidden returns forbidden error
func SafeForbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{"error": msg})
}

// IsSensitiveError checks if an error message contains sensitive information
func IsSensitiveError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())

	sensitivePatterns := []string{
		// Database
		"postgres", "mysql", "sqlite", "database", "sql",
		"connection", "connect", "failed to connect",
		// Network
		"dial", "tcp", "udp", "socket", "timeout",
		// Server info
		"127.0.0.1", "localhost", "0.0.0.0",
		// File system
		"no such file", "permission denied", "open /",
		// Credentials
		"password", "user=", "host=", "port=",
		// Internal
		"panic", "runtime error", "stack trace",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Check for IP addresses (simple pattern)
	if strings.Contains(errMsg, ":") && (strings.Contains(errMsg, ".") || strings.Contains(errMsg, "::")) {
		return true
	}

	return false
}

// SanitizeError returns the error message if safe, otherwise returns a generic message
func SanitizeError(err error, fallbackMsg string) string {
	if err == nil {
		return fallbackMsg
	}
	if IsSensitiveError(err) {
		return fallbackMsg
	}
	return err.Error()
}
