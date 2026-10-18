package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"nofx/logger"
)

const factorMiningMaxFileSize = 8 << 20 // 8 MiB per file, including runs.jsonl.

func factorMiningDir() string {
	if dir := os.Getenv("FACTOR_MINING_DIR"); dir != "" {
		return dir
	}
	return "data/factor_mining"
}

// readFactorMiningFile only receives server-defined paths, never request input.
// Limit the read as well as checking stat, since the writer may append meanwhile.
func readFactorMiningFile(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnf("[factor-mining] Cannot open %s: %v", path, err)
		}
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		logger.Warnf("[factor-mining] Cannot stat %s: %v", path, err)
		return nil
	}
	if info.Size() > factorMiningMaxFileSize {
		logger.Warnf("[factor-mining] Ignoring %s: exceeds 8 MiB", path)
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, factorMiningMaxFileSize+1))
	if err != nil {
		logger.Warnf("[factor-mining] Cannot read %s: %v", path, err)
		return nil
	}
	if len(data) > factorMiningMaxFileSize {
		logger.Warnf("[factor-mining] Ignoring %s: exceeds 8 MiB", path)
		return nil
	}
	return data
}

func readFactorMiningJSON(path string) json.RawMessage {
	data := readFactorMiningFile(path)
	if data == nil {
		return nil
	}
	if !json.Valid(data) {
		logger.Warnf("[factor-mining] Invalid JSON in %s", path)
		return nil
	}
	return json.RawMessage(data)
}

func (s *Server) handleFactorMiningOverview(c *gin.Context) {
	dir := factorMiningDir()
	c.JSON(http.StatusOK, gin.H{
		"status": readFactorMiningJSON(filepath.Join(dir, "status.json")),
		"zoo": gin.H{
			"crypto": readFactorMiningJSON(filepath.Join(dir, "crypto", "zoo.json")),
			"us":     readFactorMiningJSON(filepath.Join(dir, "us", "zoo.json")),
		},
		"ensemble": gin.H{
			"crypto": readFactorMiningJSON(filepath.Join(dir, "crypto", "ensemble.json")),
			"us":     readFactorMiningJSON(filepath.Join(dir, "us", "ensemble.json")),
		},
	})
}

func (s *Server) handleFactorMiningRuns(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	runs := make([]json.RawMessage, 0, limit)
	path := filepath.Join(factorMiningDir(), "runs.jsonl")
	if data := readFactorMiningFile(path); len(data) > 0 {
		// A terminal newline ends the last record; it is not an extra line.
		data = bytes.TrimSuffix(data, []byte("\n"))
		// The log is append-only: reverse the last limit physical lines to
		// return newest first. Bad lines do not pull older records into view.
		skipped := 0
		for seen, end := 0, len(data); seen < limit && end > 0; seen++ {
			start := bytes.LastIndexByte(data[:end], '\n') + 1
			line := bytes.TrimSpace(data[start:end])
			end = start - 1
			if !json.Valid(line) {
				skipped++
				continue
			}
			runs = append(runs, json.RawMessage(line))
		}
		if skipped > 0 {
			logger.Warnf("[factor-mining] Skipped %d invalid JSON lines in %s", skipped, path)
		}
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}
