package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"nofx/logger"
)

const promotionMaxFileSize = 8 << 20 // 8 MiB per report.

func promotionDir() string {
	if dir := os.Getenv("PROMOTION_DIR"); dir != "" {
		return dir
	}
	return "data/promotion"
}

// readPromotionFile only receives a server-defined path, never request input.
// Bound both the initial size and the read in case the file grows meanwhile.
func readPromotionFile(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnf("[promotion] Cannot open %s: %v", path, err)
		}
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		logger.Warnf("[promotion] Cannot stat %s: %v", path, err)
		return nil
	}
	if info.Size() > promotionMaxFileSize {
		logger.Warnf("[promotion] Ignoring %s: exceeds 8 MiB", path)
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, promotionMaxFileSize+1))
	if err != nil {
		logger.Warnf("[promotion] Cannot read %s: %v", path, err)
		return nil
	}
	if len(data) > promotionMaxFileSize {
		logger.Warnf("[promotion] Ignoring %s: exceeds 8 MiB", path)
		return nil
	}
	return data
}

func (s *Server) handlePromotionReport(c *gin.Context) {
	path := filepath.Join(promotionDir(), "report.json")
	data := readPromotionFile(path)
	if data != nil && !json.Valid(data) {
		logger.Warnf("[promotion] Invalid JSON in %s", path)
		data = nil
	}
	if data == nil {
		data = []byte(`{"pairs":[],"generated_at":null}`)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}
