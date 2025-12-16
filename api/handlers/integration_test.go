package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
	"nofx/config"
)

func setupTestHandler(t *testing.T) (*BaseHandler, *gin.Engine) {
	// Setup in-memory SQLite DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}

	// Initialize schema (simplified for test)
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT false,
			locked_until TIMESTAMP,
			failed_attempts INTEGER DEFAULT 0,
			last_failed_at TIMESTAMP,
			is_active BOOLEAN DEFAULT true,
			is_admin BOOLEAN DEFAULT false,
			beta_code TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Use our helper to create a config.Database from the sqlite connection
	// Note: config.NewTestDatabase must be available (created in previous step)
	database := config.NewTestDatabase(db)

	h := &BaseHandler{
		Database: database,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()

	return h, r
}

func TestHandleRegister_InvalidInput(t *testing.T) {
	h, r := setupTestHandler(t)
	r.POST("/register", h.HandleRegister)

	payload := map[string]string{
		"email":    "bad-email",
		"password": "short",
	}
	jsonValue, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonValue))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetSystemConfig(t *testing.T) {
	h, r := setupTestHandler(t)
	r.GET("/config", h.HandleGetSystemConfig)

	// Pre-populate some config
	h.Database.SetSystemConfig("beta_mode", "true")

	req, _ := http.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, true, response["beta_mode"])
}
