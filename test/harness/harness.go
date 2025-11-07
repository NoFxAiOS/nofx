package harness

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"nofx/api"
	"nofx/auth"
	"nofx/config"
	"nofx/manager"

	_ "github.com/mattn/go-sqlite3"
)

// TestEnv 测试环境，持有临时DB、API server 等
type TestEnv struct {
	DBPath        string
	Database      *config.Database
	APIServer     *httptest.Server
	APIServerURL  string
	TraderManager *manager.TraderManager
}

// NewTestEnv 创建一个新的测试环境
func NewTestEnv(t *testing.T) *TestEnv {
	// 创建临时目录
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// 创建数据库
	db, err := config.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}

	// set a deterministic JWT secret for tests and enable admin mode to avoid Authorization headers
	auth.SetJWTSecret("test-jwt-secret")
	auth.SetAdminMode(true)

	// 创建TraderManager
	traderMgr := manager.NewTraderManager()

	// 创建API server
	apiServer := api.NewServer(traderMgr, db, 0)
	server := httptest.NewServer(apiServer.Router())

	return &TestEnv{
		DBPath:        dbPath,
		Database:      db,
		APIServer:     server,
		APIServerURL:  server.URL,
		TraderManager: traderMgr,
	}
}

// Cleanup 关闭环境
func (e *TestEnv) Cleanup() {
	// reset admin mode
	auth.SetAdminMode(false)

	if e.APIServer != nil {
		e.APIServer.Close()
	}
	if e.Database != nil {
		e.Database.Close()
	}
	// remove db file if exists
	os.Remove(e.DBPath)
}

// MustExecSQL 在测试DB上执行SQL，若失败则Fatal
func (e *TestEnv) MustExecSQL(t *testing.T, query string, args ...any) {
	db := e.Database
	if db == nil {
		t.Fatalf("Database is nil")
	}
	_, err := db.DB().Exec(query, args...)
	if err != nil {
		t.Fatalf("ExecSQL failed: %v", err)
	}
}

// DB returns underlying *sql.DB
func (e *TestEnv) DB() *sql.DB {
	return e.Database.DB()
}

// URL returns API base URL
func (e *TestEnv) URL() string { return e.APIServerURL }

// ValidateDBWithCSV 从 CSV 文件加载验证规则并执行校验与清理
// CSV 格式： table,column,value,flag
// flag: c = 用作 WHERE 条件; Y = 需要校验; n = 跳过
// value 使用 '*' 表示非空校验
func (e *TestEnv) ValidateDBWithCSV(t *testing.T, csvPath string) {
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv failed: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}

	if len(records) == 0 {
		t.Fatalf("csv is empty: %s", csvPath)
	}

	// optional header detection: if first row looks like 'table,column,value,flag' skip it
	if len(records) > 0 {
		head := records[0]
		if len(head) >= 4 && strings.ToLower(strings.TrimSpace(head[0])) == "table" && strings.ToLower(strings.TrimSpace(head[1])) == "column" {
			records = records[1:]
		}
	}

	// group by table
	tableChecks := map[string][][4]string{}
	for _, rec := range records {
		// ensure 4 columns
		for len(rec) < 4 {
			rec = append(rec, "")
		}
		tbl := strings.TrimSpace(rec[0])
		col := strings.TrimSpace(rec[1])
		val := strings.TrimSpace(rec[2])
		flag := strings.TrimSpace(rec[3])
		if tbl == "" {
			t.Fatalf("csv row missing table: %v", rec)
		}
		tableChecks[tbl] = append(tableChecks[tbl], [4]string{tbl, col, val, flag})
	}

	validCol := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	db := e.DB()

	for tbl, rows := range tableChecks {
		// build where from all rows where flag==c
		whereParts := make([]string, 0)
		whereArgs := make([]interface{}, 0)
		// extra validation parts from flag==Y
		extraParts := make([]string, 0)
		extraArgs := make([]interface{}, 0)

		for _, r := range rows {
			col := r[1]
			val := r[2]
			flag := strings.ToLower(r[3])
			if col == "" {
				continue
			}
			if !validCol.MatchString(col) {
				t.Fatalf("invalid column name in csv: %s", col)
			}
			if flag == "c" {
				whereParts = append(whereParts, fmt.Sprintf("%s = ?", col))
				whereArgs = append(whereArgs, val)
			} else if flag == "y" || flag == "yes" || flag == "1" {
				if val == "*" {
					extraParts = append(extraParts, fmt.Sprintf("%s IS NOT NULL AND %s != ''", col, col))
				} else {
					extraParts = append(extraParts, fmt.Sprintf("%s = ?", col))
					extraArgs = append(extraArgs, val)
				}
			}
		}

		if len(whereParts) == 0 {
			t.Fatalf("no WHERE condition (flag=c) specified for table %s in csv %s", tbl, csvPath)
		}

		whereClause := strings.Join(whereParts, " AND ")
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tbl, whereClause)
		if len(extraParts) > 0 {
			query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND (%s)", tbl, whereClause, strings.Join(extraParts, " AND "))
		}

		fullArgs := make([]interface{}, 0)
		fullArgs = append(fullArgs, whereArgs...)
		fullArgs = append(fullArgs, extraArgs...)

		var cnt int
		err = db.QueryRow(query, fullArgs...).Scan(&cnt)
		if err != nil {
			t.Fatalf("db query failed: %v; query: %s", err, query)
		}
		if cnt == 0 {
			// Try to fetch a sample row using only WHERE (without extraParts) to show actual values for debugging
			sampleQuery := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 1", tbl, whereClause)
			rows, err := db.Query(sampleQuery, whereArgs...)
			if err == nil {
				defer rows.Close()
				if rows.Next() {
					cols, _ := rows.Columns()
					vals := make([]interface{}, len(cols))
					ptrs := make([]interface{}, len(cols))
					for i := range vals {
						ptrs[i] = &vals[i]
					}
					_ = rows.Scan(ptrs...)
					pairs := make([]string, 0, len(cols))
					for i, c := range cols {
						var s string
						if vals[i] != nil {
							s = fmt.Sprintf("%v", vals[i])
						} else {
							s = ""
						}
						pairs = append(pairs, fmt.Sprintf("%s=%s", c, s))
					}
					// 此处函数签名不返回 error，使用 t.Fatalf 报错并停止测试
					t.Fatalf("validation failed for table %s: query returned 0 rows. query=%s args=%v; sample row: %s", tbl, query, fullArgs, strings.Join(pairs, ", "))
				}
			}
			// 函数签名不返回 error，使用 t.Fatalf
			t.Fatalf("validation failed for table %s: query returned 0 rows. query=%s args=%v", tbl, query, fullArgs)
		}

		// cleanup using only whereArgs (from flag=c)
		delQuery := fmt.Sprintf("DELETE FROM %s WHERE %s", tbl, whereClause)
		if _, err := db.Exec(delQuery, whereArgs...); err != nil {
			t.Fatalf("cleanup delete failed: %v; query: %s", err, delQuery)
		}
	}
}

// CaseSpec 表示 case.yml 中的结构
type CaseSpec struct {
	Name    string `yaml:"name"`
	Request struct {
		Method  string                 `yaml:"method"`
		Path    string                 `yaml:"path"`
		Headers map[string]string      `yaml:"headers,omitempty"`
		Body    map[string]interface{} `yaml:"body,omitempty"`
	} `yaml:"request"`
	Expect struct {
		Status int                    `yaml:"status"`
		Body   map[string]interface{} `yaml:"body,omitempty"`
	} `yaml:"expect"`
}

// RunAnnotatedCases 在被测测试文件中查找 @RunWith(...) 注解并执行匹配的用例目录
// 调用位置：在测试函数中调用 harness.RunAnnotatedCases(t)
func RunAnnotatedCases(t *testing.T) {
	// locate caller test file
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	// base dir is directory of the test file
	baseDir := filepath.Dir(file)

	// read source to find @RunWith(...) annotations
	src, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("read test file failed: %v", err)
	}
	// regex to capture contents inside @RunWith(...)
	r := regexp.MustCompile(`@RunWith\((.*?)\)`) // non-greedy
	matches := r.FindAllStringSubmatch(string(src), -1)
	var patterns []string
	for _, m := range matches {
		if len(m) >= 2 {
			patterns = append(patterns, strings.TrimSpace(m[1]))
		}
	}

	// detect optional @Target(...) annotation to specify the tested method
	// formats supported: @Target(handleRegister) or @Target(api/server.go:handleRegister)
	targetRe := regexp.MustCompile(`@Target\((.*?)\)`) // capture inside
	targetMatch := targetRe.FindStringSubmatch(string(src))
	if len(targetMatch) >= 2 {
		raw := strings.TrimSpace(targetMatch[1])
		// extract function name after optional ':'
		parts := strings.Split(raw, ":")
		funcName := parts[len(parts)-1]
		funcName = strings.Trim(funcName, ` \"'`)
		// try to resolve repo root (folder containing go.mod)
		repoRoot := resolveRepoRoot(filepath.Dir(file))
		if repoRoot != "" {
			cand := filepath.Join(repoRoot, "test", funcName)
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				baseDir = cand
			}
		}
	}

	// if no annotations, run all cases under baseDir
	caseDirs, err := ioutil.ReadDir(baseDir)
	if err != nil {
		t.Fatalf("read baseDir failed: %v", err)
	}

	// build list of candidate case directories
	candidates := make([]string, 0)
	for _, fi := range caseDirs {
		if fi.IsDir() && strings.HasPrefix(fi.Name(), "case") {
			candidates = append(candidates, filepath.Join(baseDir, fi.Name()))
		}
	}

	// filter by patterns if present (patterns may be regex or comma-separated)
	selected := make([]string, 0)
	if len(patterns) == 0 {
		selected = candidates
	} else {
		for _, p := range patterns {
			// support comma separated
			parts := strings.Split(p, ",")
			for _, part := range parts {
				part = strings.Trim(part, ` \"'`)
				if part == "*" || part == ".*" {
					selected = append(selected, candidates...)
					continue
				}
				// treat as regex
				re, err := regexp.Compile(part)
				if err != nil {
					// treat as literal name
					for _, c := range candidates {
						if strings.HasSuffix(c, part) || strings.HasSuffix(c, filepath.Join("", part)) || strings.HasSuffix(c, "/"+part) {
							selected = append(selected, c)
						}
					}
					continue
				}
				for _, c := range candidates {
					if re.MatchString(filepath.Base(c)) {
						selected = append(selected, c)
					}
				}
			}
		}
	}

	if len(selected) == 0 {
		t.Skip("no matching cases found")
	}

	// run each selected case as subtest
	for _, caseDir := range selected {
		caseName := filepath.Base(caseDir)
		t.Run(caseName, func(t *testing.T) {
			runSingleCase(t, caseDir)
		})
	}
}

// runSingleCase 执行单个用例目录里的准备、请求、断言与db检查
func runSingleCase(t *testing.T, caseDir string) {
	// create new env per case for isolation and delegate to runSingleCaseWithEnv
	env := NewTestEnv(t)
	defer env.Cleanup()

	runSingleCaseWithEnv(t, caseDir, env)
}

// runSingleCaseWithEnv 执行单个用例目录里的准备、请求、断言与db检查，使用传入的 env （不创建新的）
func runSingleCaseWithEnv(t *testing.T, caseDir string, env *TestEnv) {
	// artifacts dir (optional, enabled by HARNESS_SAVE_ARTIFACTS=1)
	artifacts := filepath.Join("test_artifacts", filepath.Base(caseDir))
	saveArtifacts := os.Getenv("HARNESS_SAVE_ARTIFACTS") == "1"

	// no longer directly create or write files here; use ReportArtifact which
	// will only write when saveArtifacts==true and otherwise logs suppressed artifacts.

	// 1. prepare data - support CSV files in caseDir/PrepareData with filename = table name
	prepDir := filepath.Join(caseDir, "PrepareData")
	if fi, err := os.Stat(prepDir); err == nil && fi.IsDir() {
		files, _ := ioutil.ReadDir(prepDir)
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !strings.HasSuffix(f.Name(), ".csv") {
				continue
			}
			table := strings.TrimSuffix(f.Name(), ".csv")
			csvPath := filepath.Join(prepDir, f.Name())
			if err := loadCSVToTable(env.DB(), csvPath, table); err != nil {
				t.Fatalf("prepare data load failed for %s: %v", csvPath, err)
			}
		}
	}

	// 2. read case.yml
	caseYml := filepath.Join(caseDir, "case.yml")
	if _, err := os.Stat(caseYml); err != nil {
		t.Fatalf("case.yml not found in %s", caseDir)
	}
	cs := &CaseSpec{}
	b, _ := ioutil.ReadFile(caseYml)
	if err := yaml.Unmarshal(b, cs); err != nil {
		t.Fatalf("parse case.yml failed: %v", err)
	}

	// 3. build and send request
	method := strings.ToUpper(cs.Request.Method)
	if method == "" {
		method = "GET"
	}
	reqBody := []byte{}
	if cs.Request.Body != nil {
		rb, _ := json.Marshal(cs.Request.Body)
		reqBody = rb
	}
	url := env.URL() + cs.Request.Path
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	for k, v := range cs.Request.Headers {
		req.Header.Set(k, v)
	}
	// If test harness runs in admin mode and caller didn't set Authorization,
	// automatically add a test JWT so protected endpoints succeed.
	if auth.IsAdminMode() && req.Header.Get("Authorization") == "" {
		if token, err := auth.GenerateJWT("admin", "admin@localhost"); err == nil {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	if len(reqBody) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	defer res.Body.Close()

	// read response body for assertions
	respBytes, _ := ioutil.ReadAll(res.Body)

	// if saving artifacts, write request/response and status via ReportArtifact
	_ = ReportArtifact(t, saveArtifacts, artifacts, "request.json", reqBody)
	_ = ReportArtifact(t, saveArtifacts, artifacts, "response.json", respBytes)
	_ = ReportArtifact(t, saveArtifacts, artifacts, "response_status.txt", []byte(fmt.Sprintf("%d", res.StatusCode)))

	// also dump users table for debugging into an artifact (suppressed by default)
	func() {
		rows, err := env.DB().Query("SELECT id, email, otp_secret, otp_verified, created_at FROM users LIMIT 100")
		if err != nil {
			// previously wrote users_dump_error.txt; now report only if saving or log when suppressed
			_ = ReportArtifact(t, saveArtifacts, artifacts, "users_dump_error.txt", []byte(err.Error()))
			return
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		list := make([]map[string]interface{}, 0)
		for rows.Next() {
			_ = rows.Scan(ptrs...)
			rowMap := make(map[string]interface{})
			for i, c := range cols {
				if vals[i] != nil {
					rowMap[c] = fmt.Sprintf("%v", vals[i])
				} else {
					rowMap[c] = nil
				}
			}
			list = append(list, rowMap)
		}
		if b2, err := json.MarshalIndent(list, "", "  "); err == nil {
			_ = ReportArtifact(t, saveArtifacts, artifacts, "users_dump.json", b2)
		}
	}()

	// 4. check response status
	if cs.Expect.Status != 0 {
		if res.StatusCode != cs.Expect.Status {
			body := string(respBytes)
			t.Fatalf("expected status %d got %d; body=%s", cs.Expect.Status, res.StatusCode, body)
		}
	}

	// 5. validate response body against expected (partial match with extended operators)
	if cs.Expect.Body != nil {
		var actual map[string]interface{}
		if len(bytes.TrimSpace(respBytes)) == 0 {
			t.Fatalf("expected response body but got empty")
		}
		if err := json.Unmarshal(respBytes, &actual); err != nil {
			t.Fatalf("parse response json failed: %v; raw=%s", err, string(respBytes))
		}
		if !matchJSONExtended(cs.Expect.Body, actual) {
			mismatches := collectMismatches(cs.Expect.Body, actual, "")
			if len(mismatches) == 0 {
				t.Fatalf("response body did not match expectation (unknown mismatch)")
			}
			// print only mismatched keys
			t.Fatalf("response body did not match expectation for keys:\n%s", strings.Join(mismatches, "\n"))
		}
	}

	// 6. DB checks: look for per-table CSVs under checkData (case-sensitive variants)
	checkDirs := collectExistingDirs(caseDir, "CheckData")
	for _, cd := range checkDirs {
		files, _ := ioutil.ReadDir(cd)
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !strings.HasSuffix(f.Name(), ".csv") {
				continue
			}
			table := strings.TrimSuffix(f.Name(), ".csv")
			csvPath := filepath.Join(cd, f.Name())
			if err := validateTableCSV(env.DB(), csvPath, table); err != nil {
				t.Fatalf("checkData failed for %s: %v", csvPath, err)
			}
		}
	}
	// if no checkData provided, OK - no DB checks
}

// collectExistingDirs checks candidate subdirectories (by name) under caseDir,
// returns a deduplicated list of absolute paths that exist and are directories.
// It resolves symlinks and uses the resolved absolute path to deduplicate, so
// on case-insensitive filesystems or when both `checkData` and `CheckData` point
// to the same directory we won't process it twice.
func collectExistingDirs(caseDir string, names ...string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, n := range names {
		cand := filepath.Join(caseDir, n)
		fi, err := os.Stat(cand)
		if err != nil || !fi.IsDir() {
			continue
		}
		abs, err := filepath.Abs(cand)
		if err != nil {
			abs = cand
		}
		real, err := filepath.EvalSymlinks(abs)
		if err == nil {
			abs = real
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out
}

// loadCSVToTable 插入 CSV 文件到指定表，支持两种格式：
// 1) Header 格式：第一行为列名，后续行是多条记录（原实现）
// 2) Key-Value 格式（和 CheckData 保持一致）：每行为 column,value,flag，用来描述单条待插入记录
func loadCSVToTable(db *sql.DB, csvPath, table string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	// Use flexible CSV parser that handles single-quoted fields containing commas
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	rows, err := parseFlexibleCSV(b)
	if err != nil {
		return err
	}
	if len(rows) < 1 {
		return nil
	}

	// detect key-value format: first row starts with 'column' (case-insensitive) and has at least 3 cols
	first := rows[0]
	if len(first) >= 1 && strings.ToLower(strings.TrimSpace(first[0])) == "column" {
		// treat as key-value pairs: each subsequent row is column,value,flag
		validCol := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
		cols := make([]string, 0)
		vals := make([]interface{}, 0)
		index := make(map[string]int)
		for i := 1; i < len(rows); i++ {
			r := rows[i]
			for len(r) < 3 {
				r = append(r, "")
			}
			col := strings.TrimSpace(r[0])
			val := strings.TrimSpace(r[1])
			flag := strings.ToLower(strings.TrimSpace(r[2]))
			if col == "" {
				continue
			}
			if !validCol.MatchString(col) {
				return fmt.Errorf("invalid column name '%s' in %s", col, csvPath)
			}
			// skip rows explicitly marked as not applicable
			if flag == "n" || flag == "no" {
				continue
			}
			if idx, ok := index[col]; ok {
				// overwrite previous value for the same column
				vals[idx] = val
			} else {
				index[col] = len(cols)
				cols = append(cols, col)
				vals = append(vals, val)
			}
		}
		if len(cols) == 0 {
			return nil
		}
		// Ensure required NOT NULL columns without defaults are provided: inspect table schema
		// PRAGMA table_info(table) returns: cid,name,type,notnull,dflt_value,pk
		rowsInfo, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err == nil {
			defer rowsInfo.Close()
			for rowsInfo.Next() {
				var cid int
				var name string
				var ctype string
				var notnull int
				var dflt sql.NullString
				var pk int
				if err := rowsInfo.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
					continue
				}
				// skip if already provided
				if _, ok := index[name]; ok {
					continue
				}
				// skip primary key columns (assume auto populated)
				if pk == 1 {
					continue
				}
				if notnull == 1 && !dflt.Valid {
					// provide a fallback value based on column type
					ct := strings.ToUpper(ctype)
					var fallback string
					if strings.Contains(ct, "INT") || strings.Contains(ct, "NUM") || strings.Contains(ct, "REAL") || strings.Contains(ct, "FLOA") {
						fallback = "0"
					} else {
						// treat as text/blob/date -> empty string
						fallback = ""
					}
					index[name] = len(cols)
					cols = append(cols, name)
					vals = append(vals, fallback)
				}
			}
		}
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","), strings.Join(placeholders, ","))
		if _, err := db.Exec(insertSQL, vals...); err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
		return nil
	}

	// fallback: header format (original behavior)
	header := rows[0]
	placeholders := make([]string, len(header))
	cols := make([]string, len(header))
	for i, c := range header {
		cols[i] = strings.TrimSpace(c)
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","), strings.Join(placeholders, ","))
	for _, row := range rows[1:] {
		vals := make([]interface{}, len(cols))
		for i := range cols {
			if i < len(row) {
				vals[i] = strings.TrimSpace(row[i])
			} else {
				vals[i] = ""
			}
		}
		if _, err := db.Exec(insertSQL, vals...); err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
	}
	return nil
}

// validateTableCSV 验证 per-table CSV 格式：每行 column,value,flag（支持可选表头 column,value,flag）
func validateTableCSV(db *sql.DB, csvPath, table string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	// use flexible parser
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	recs, err := parseFlexibleCSV(b)
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		return fmt.Errorf("empty csv %s", csvPath)
	}
	// detect header: first cell equals 'column' (case-insensitive)
	start := 0
	if len(recs[0]) >= 1 && strings.ToLower(strings.TrimSpace(recs[0][0])) == "column" {
		start = 1
	}

	validCol := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

	// Build WHERE only from flag == c rows
	whereParts := []string{}
	whereArgs := []interface{}{}
	// Keep expected assertions for later
	expects := make([][3]string, 0)

	for i := start; i < len(recs); i++ {
		r := recs[i]
		if len(r) < 3 {
			for len(r) < 3 {
				r = append(r, "")
			}
		}
		col := strings.TrimSpace(r[0])
		val := strings.TrimSpace(r[1])
		flag := strings.ToLower(strings.TrimSpace(r[2]))
		if col == "" {
			return fmt.Errorf("empty column in %s row %d", csvPath, i+1)
		}
		if !validCol.MatchString(col) {
			return fmt.Errorf("invalid column name '%s' in %s", col, csvPath)
		}
		if flag == "c" {
			whereParts = append(whereParts, fmt.Sprintf("%s = ?", col))
			whereArgs = append(whereArgs, val)
		} else if flag == "y" || flag == "yes" || flag == "1" {
			expects = append(expects, [3]string{col, val, flag})
		} else {
			// skip n or empty
		}
	}

	if len(whereParts) == 0 {
		return fmt.Errorf("no WHERE condition (flag=c) specified in %s", csvPath)
	}

	whereClause := strings.Join(whereParts, " AND ")
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 1", table, whereClause)
	row := db.QueryRow(query, whereArgs...)
	cols, err := func() ([]string, error) {
		// To get column names, run the query and use Rows.Columns
		rows, err := db.Query(query, whereArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		if !rows.Next() {
			return nil, fmt.Errorf("no matching row for where in %s: %v", csvPath, whereArgs)
		}
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		// prepare scan
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		// build map
		// convert values to string map for assertions
		// but return cols and store scanned values below by re-query using row.Scan again
		return cols, nil
	}()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Now fetch the row values again using QueryRow and scan into interface slice
	cols2 := cols
	vals2 := make([]interface{}, len(cols2))
	ptrs2 := make([]interface{}, len(cols2))
	for i := range vals2 {
		ptrs2[i] = &vals2[i]
	}
	if err := row.Scan(ptrs2...); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	data := map[string]string{}
	for i, c := range cols2 {
		if vals2[i] != nil {
			data[c] = fmt.Sprintf("%v", vals2[i])
		} else {
			data[c] = ""
		}
	}

	// perform expectations checks
	for _, exp := range expects {
		col := exp[0]
		val := exp[1]
		// operator handling same as validateTableCSV previous
		actual := data[col]
		if strings.HasPrefix(val, "RE:") {
			pat := strings.TrimPrefix(val, "RE:")
			matched, _ := regexp.MatchString(pat, actual)
			if !matched {
				return fmt.Errorf("regex assert failed on %s: pattern=%s actual=%s", col, pat, actual)
			}
		} else if strings.HasPrefix(val, "NEQ:") {
			v := strings.TrimPrefix(val, "NEQ:")
			if actual == v {
				return fmt.Errorf("not-equal assert failed on %s: actual=%s", col, actual)
			}
		} else if strings.HasPrefix(val, "IN:") {
			list := strings.Split(strings.TrimPrefix(val, "IN:"), "|")
			ok := false
			for _, v := range list {
				if actual == v {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("in assert failed on %s: actual=%s expected in %v", col, actual, list)
			}
		} else if strings.EqualFold(val, "EXISTS:true") || val == "*" {
			if actual == "" {
				return fmt.Errorf("exists assert failed on %s: actual empty", col)
			}
		} else {
			// exact match with normalization
			if !compareNormalized(actual, val) {
				return fmt.Errorf("assert failed on %s: expected=%s actual=%s", col, val, actual)
			}
		}
	}

	// cleanup using only whereArgs
	delQuery := fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause)
	if _, err := db.Exec(delQuery, whereArgs...); err != nil {
		return fmt.Errorf("cleanup delete failed: %w; query=%s", err, delQuery)
	}
	return nil
}

// compareNormalized compares actual and expected with basic normalization for booleans and numbers
func compareNormalized(actual, expected string) bool {
	a := strings.TrimSpace(actual)
	e := strings.TrimSpace(expected)
	// normalize boolean numeric expectations
	if e == "0" || e == "1" {
		if a == "true" || a == "false" {
			if e == "0" && a == "false" {
				return true
			}
			if e == "1" && a == "true" {
				return true
			}
		}
	}
	// normalize '0'/'1' actual to booleans
	if a == "0" || a == "1" {
		if e == "false" && a == "0" {
			return true
		}
		if e == "true" && a == "1" {
			return true
		}
	}
	// numeric compare: if both parse as numbers, compare numeric equality
	if _, errA := strconv.ParseFloat(a, 64); errA == nil {
		if _, errE := strconv.ParseFloat(e, 64); errE == nil {
			na, _ := strconv.ParseFloat(a, 64)
			ne, _ := strconv.ParseFloat(e, 64)
			return na == ne
		}
	}
	// fallback to string equality
	return a == e
}

// matchJSONExtended supports special operators in expected map values: strings starting with RE:, NEQ:, IN:, EXISTS:true, and supports nested maps/arrays
func matchJSONExtended(expected, actual map[string]interface{}) bool {
	for k, v := range expected {
		av, ok := actual[k]
		if !ok {
			return false
		}
		switch ev := v.(type) {
		case string:
			if strings.HasPrefix(ev, "RE:") {
				pat := strings.TrimPrefix(ev, "RE:")
				s := fmt.Sprintf("%v", av)
				matched, _ := regexp.MatchString(pat, s)
				if !matched {
					return false
				}
			} else if strings.HasPrefix(ev, "NEQ:") {
				s := fmt.Sprintf("%v", av)
				if s == strings.TrimPrefix(ev, "NEQ:") {
					return false
				}
			} else if strings.HasPrefix(ev, "IN:") {
				list := strings.Split(strings.TrimPrefix(ev, "IN:"), "|")
				s := fmt.Sprintf("%v", av)
				ok2 := false
				for _, item := range list {
					if s == item {
						ok2 = true
						break
					}
				}
				if !ok2 {
					return false
				}
			} else if ev == "*" || ev == "EXISTS:true" {
				s := fmt.Sprintf("%v", av)
				if s == "" {
					return false
				}
			} else {
				if fmt.Sprintf("%v", av) != ev {
					return false
				}
			}
		case map[string]interface{}:
			if avMap, ok := av.(map[string]interface{}); ok {
				if !matchJSONExtended(ev, avMap) {
					return false
				}
			} else {
				return false
			}
		case []interface{}:
			if avArr, ok := av.([]interface{}); ok {
				for _, expItem := range ev {
					found := false
					for _, actItem := range avArr {
						if fmt.Sprintf("%v", actItem) == fmt.Sprintf("%v", expItem) {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				}
			} else {
				return false
			}
		default:
			if fmt.Sprintf("%v", av) != fmt.Sprintf("%v", ev) {
				return false
			}
		}
	}
	return true
}

// collectMismatches recursively compares expected vs actual and returns a list of human-readable mismatch messages for
// only the keys that do not satisfy the expectation. prefix is used for nested keys (dot-separated).
func collectMismatches(expected, actual map[string]interface{}, prefix string) []string {
	mismatches := make([]string, 0)
	for k, v := range expected {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		av, ok := actual[k]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("%s: missing (expected=%v)", fullKey, v))
			continue
		}
		switch ev := v.(type) {
		case string:
			sAct := fmt.Sprintf("%v", av)
			if strings.HasPrefix(ev, "RE:") {
				pat := strings.TrimPrefix(ev, "RE:")
				matched, _ := regexp.MatchString(pat, sAct)
				if !matched {
					mismatches = append(mismatches, fmt.Sprintf("%s: regex mismatch expected=%s actual=%s", fullKey, ev, sAct))
				}
			} else if strings.HasPrefix(ev, "NEQ:") {
				exp := strings.TrimPrefix(ev, "NEQ:")
				if sAct == exp {
					mismatches = append(mismatches, fmt.Sprintf("%s: not-equal assertion failed (actual=%s should not equal %s)", fullKey, sAct, exp))
				}
			} else if strings.HasPrefix(ev, "IN:") {
				list := strings.Split(strings.TrimPrefix(ev, "IN:"), "|")
				ok2 := false
				for _, item := range list {
					if sAct == item {
						ok2 = true
						break
					}
				}
				if !ok2 {
					mismatches = append(mismatches, fmt.Sprintf("%s: in-assert failed actual=%s expected one of=%v", fullKey, sAct, list))
				}
			} else if ev == "*" || ev == "EXISTS:true" {
				if sAct == "" {
					mismatches = append(mismatches, fmt.Sprintf("%s: expected to exist and be non-empty, actual empty", fullKey))
				}
			} else {
				if sAct != ev {
					mismatches = append(mismatches, fmt.Sprintf("%s: expected=%s actual=%s", fullKey, ev, sAct))
				}
			}
		case map[string]interface{}:
			if avMap, ok := av.(map[string]interface{}); ok {
				rec := collectMismatches(ev, avMap, fullKey)
				mismatches = append(mismatches, rec...)
			} else {
				mismatches = append(mismatches, fmt.Sprintf("%s: type mismatch expected map actual=%T", fullKey, av))
			}
		case []interface{}:
			if avArr, ok := av.([]interface{}); ok {
				// for arrays, check each expected element appears in actual array
				for _, expItem := range ev {
					expStr := fmt.Sprintf("%v", expItem)
					found := false
					for _, actItem := range avArr {
						if fmt.Sprintf("%v", actItem) == expStr {
							found = true
							break
						}
					}
					if !found {
						mismatches = append(mismatches, fmt.Sprintf("%s: array missing expected element=%s", fullKey, expStr))
					}
				}
			} else {
				mismatches = append(mismatches, fmt.Sprintf("%s: type mismatch expected array actual=%T", fullKey, av))
			}
		default:
			if fmt.Sprintf("%v", av) != fmt.Sprintf("%v", ev) {
				mismatches = append(mismatches, fmt.Sprintf("%s: expected=%v actual=%v", fullKey, ev, av))
			}
		}
	}
	return mismatches
}

// GenerateCaseYAML 帮助生成 case.yml 模板到 caseDir
func GenerateCaseYAML(caseDir, name, method, path string, reqBody map[string]interface{}, expectStatus int, expectBody map[string]interface{}) error {
	cs := &CaseSpec{Name: name}
	cs.Request.Method = method
	cs.Request.Path = path
	cs.Request.Body = reqBody
	cs.Expect.Status = expectStatus
	cs.Expect.Body = expectBody
	b, err := yaml.Marshal(cs)
	if err != nil {
		return err
	}
	os.MkdirAll(caseDir, 0o755)
	return ioutil.WriteFile(filepath.Join(caseDir, "case.yml"), b, 0o644)
}

// resolveRepoRoot walks up from startDir to find a directory containing go.mod and returns it
func resolveRepoRoot(startDir string) string {
	cur := startDir
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur || parent == "." || parent == "" {
			break
		}
		cur = parent
	}
	return ""
}

// BaseTest 提供可重写的 Before/After 钩子，并持有 TestEnv
type BaseTest struct {
	Env *TestEnv
}

func (b *BaseTest) Before(t *testing.T) {}
func (b *BaseTest) After(t *testing.T)  {}

// SetEnv 将 TestEnv 注入到 BaseTest（供 RunCase 调用）
func (b *BaseTest) SetEnv(env *TestEnv) { b.Env = env }

// TestCase 表示一个可运行的集成测试用例，包含在运行前后的钩子
type TestCase interface {
	SetEnv(env *TestEnv)
	Before(t *testing.T)
	After(t *testing.T)
}

// RunCase 在测试函数中调用：它会解析调用者文件上的 @RunWith(...) 和 @Target(...) 注解，
// 找到对应的 case 目录并为每个 case运行 SetEnv -> Before -> runSingleCaseWithEnv -> After -> Cleanup 的流程。
func RunCase(t *testing.T, tc TestCase) {
	// locate caller test file (the test that called RunCase)
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	baseDir := filepath.Dir(file)

	// read source to find @RunWith and @Target
	src, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("read test file failed: %v", err)
	}
	// capture @RunWith
	r := regexp.MustCompile(`@RunWith\((.*?)\)`) // non-greedy
	matches := r.FindAllStringSubmatch(string(src), -1)
	var patterns []string
	for _, m := range matches {
		if len(m) >= 2 {
			patterns = append(patterns, strings.TrimSpace(m[1]))
		}
	}
	// capture @Target
	targetRe := regexp.MustCompile(`@Target\((.*?)\)`) // capture inside
	targetMatch := targetRe.FindStringSubmatch(string(src))
	if len(targetMatch) >= 2 {
		raw := strings.TrimSpace(targetMatch[1])
		parts := strings.Split(raw, ":")
		funcName := parts[len(parts)-1]
		funcName = strings.Trim(funcName, ` \"'`)
		repoRoot := resolveRepoRoot(filepath.Dir(file))
		if repoRoot != "" {
			cand := filepath.Join(repoRoot, "test", funcName)
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				baseDir = cand
			}
		}
	}

	// list case directories under baseDir
	caseDirs, err := ioutil.ReadDir(baseDir)
	if err != nil {
		t.Fatalf("read baseDir failed: %v", err)
	}
	candidates := make([]string, 0)
	for _, fi := range caseDirs {
		if fi.IsDir() && strings.HasPrefix(fi.Name(), "case") {
			candidates = append(candidates, filepath.Join(baseDir, fi.Name()))
		}
	}

	// filter by patterns if present
	selected := make([]string, 0)
	if len(patterns) == 0 {
		selected = candidates
	} else {
		for _, p := range patterns {
			parts := strings.Split(p, ",")
			for _, part := range parts {
				part = strings.Trim(part, ` \"'`)
				if part == "*" || part == ".*" {
					selected = append(selected, candidates...)
					continue
				}
				rex, err := regexp.Compile(part)
				if err != nil {
					for _, c := range candidates {
						if strings.HasSuffix(c, part) || strings.HasSuffix(c, filepath.Join("", part)) || strings.HasSuffix(c, "/"+part) {
							selected = append(selected, c)
						}
					}
					continue
				}
				for _, c := range candidates {
					if rex.MatchString(filepath.Base(c)) {
						selected = append(selected, c)
					}
				}
			}
		}
	}

	if len(selected) == 0 {
		t.Skip("no matching cases found")
	}

	// run each selected case as subtest with SetEnv/Before/After
	for _, caseDir := range selected {
		caseName := filepath.Base(caseDir)
		t.Run(caseName, func(t *testing.T) {
			// create a fresh env per case
			env := NewTestEnv(t)
			defer env.Cleanup()

			// inject env into test case
			if tc != nil {
				tc.SetEnv(env)
				// call Before hook
				tc.Before(t)
			}

			// run the case using the provided env
			runSingleCaseWithEnv(t, caseDir, env)

			if tc != nil {
				// call After hook
				tc.After(t)
			}
		})
	}
}

// ReportArtifact writes test artifacts only when saveArtifacts is true.
// When suppressed it previously logged the artifact path and size via t.Logf; that
// caused noisy logs. Default is now silent. To re-enable logging of suppressed
// artifacts set HARNESS_LOG_ARTIFACTS=1 in the environment.
func ReportArtifact(t *testing.T, saveArtifacts bool, artifactsDir, name string, data []byte) error {
	if !saveArtifacts {
		// silent by default to avoid noisy test logs. If you explicitly want to
		// see suppressed artifact messages, set HARNESS_LOG_ARTIFACTS=1.
		if os.Getenv("HARNESS_LOG_ARTIFACTS") == "1" {
			if t != nil {
				t.Logf("artifact suppressed: %s (len=%d)", filepath.Join(artifactsDir, name), len(data))
			}
		}
		return nil
	}
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		if t != nil {
			t.Logf("failed to create artifacts dir %s: %v", artifactsDir, err)
		}
		return err
	}
	path := filepath.Join(artifactsDir, name)
	if err := ioutil.WriteFile(path, data, 0o644); err != nil {
		if t != nil {
			t.Logf("failed to write artifact %s: %v", path, err)
		}
		return err
	}
	return nil
}

// ReportErrOnFailure reports an error to the test and returns the same error.
func ReportErrOnFailure(t *testing.T, ctx string, err error) error {
	if err != nil {
		if t != nil {
			t.Errorf("%s error: %v", ctx, err)
		}
	}
	return err
}

// parseFlexibleCSV parses CSV bytes while normalizing single-quoted fields to double-quoted so
// fields like 'BTCUSDT,ETHUSDT' are treated as single fields by encoding/csv.
func parseFlexibleCSV(b []byte) ([][]string, error) {
	// replace single-quoted fields '...'(no nested single quotes) with double-quoted "..."
	// This helps encoding/csv handle commas inside single-quoted fields.
	re := regexp.MustCompile(`'([^']*)'`)
	norm := re.ReplaceAllStringFunc(string(b), func(s string) string {
		// If the matched string already contains a double quote, leave it alone to avoid corrupting data
		if strings.Contains(s, `"`) {
			return s
		}
		// replace surrounding single quotes with double quotes
		inner := s[1 : len(s)-1]
		// escape any existing double quotes inside inner
		inner = strings.ReplaceAll(inner, `"`, `"`)
		return `"` + inner + `"`
	})
	r := csv.NewReader(strings.NewReader(norm))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	return r.ReadAll()
}
