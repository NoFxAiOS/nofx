package logger

import (
    "log"
    "os"
    "strings"
    "sync"
    "time"
)

// Level 日志级别
type Level int

const (
    DEBUG Level = iota
    INFO
    WARN
    ERROR
    NONE
)

var levelNames = map[string]Level{
    "debug": DEBUG,
    "info":  INFO,
    "warn":  WARN,
    "error": ERROR,
    "none":  NONE,
}

type runtimeLogger struct {
    mu             sync.RWMutex
    level          Level
    includeModules map[string]bool // 为空表示不过滤
    // 简单去抖：相同key的日志在窗口期内仅打印一次
    lastLog map[string]time.Time
    window  time.Duration
}

var rl = &runtimeLogger{
    level:   INFO,
    lastLog: make(map[string]time.Time),
    window:  3 * time.Second,
}

// InitFromEnv 读取环境变量初始化日志配置
// LOG_LEVEL=debug|info|warn|error|none
// LOG_MODULES=api,manager,market,trader
// LOG_RATE_SECONDS=3
func InitFromEnv() {
    lvl := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
    if l, ok := levelNames[lvl]; ok {
        rl.level = l
    }

    if mods := strings.TrimSpace(os.Getenv("LOG_MODULES")); mods != "" {
        rl.includeModules = make(map[string]bool)
        for _, m := range strings.Split(mods, ",") {
            m = strings.TrimSpace(m)
            if m != "" {
                rl.includeModules[strings.ToLower(m)] = true
            }
        }
    }

    if s := strings.TrimSpace(os.Getenv("LOG_RATE_SECONDS")); s != "" {
        if d, err := time.ParseDuration(s + "s"); err == nil {
            rl.window = d
        }
    }
}

func shouldLog(l Level, module string) bool {
    rl.mu.RLock()
    defer rl.mu.RUnlock()
    if l < rl.level {
        return false
    }
    if rl.includeModules == nil || len(rl.includeModules) == 0 {
        return true
    }
    _, ok := rl.includeModules[strings.ToLower(module)]
    return ok
}

func logf(l Level, module, format string, v ...interface{}) {
    if !shouldLog(l, module) {
        return
    }
    // 去抖键：级别+模块+格式（不含参数）
    key := strings.ToLower(module) + "|" + format + "|" + string('0'+l)
    if rl.window > 0 {
        rl.mu.Lock()
        if t, ok := rl.lastLog[key]; ok && time.Since(t) < rl.window {
            rl.mu.Unlock()
            return
        }
        rl.lastLog[key] = time.Now()
        rl.mu.Unlock()
    }

    prefix := ""
    switch l {
    case DEBUG:
        prefix = "[DEBUG]"
    case INFO:
        prefix = "[INFO]"
    case WARN:
        prefix = "[WARN]"
    case ERROR:
        prefix = "[ERROR]"
    }
    if module != "" {
        prefix += "[" + module + "] "
    } else {
        prefix += " "
    }
    log.Printf(prefix+format, v...)
}

// 对外暴露的便捷函数
func Debugf(module, format string, v ...interface{}) { logf(DEBUG, module, format, v...) }
func Infof(module, format string, v ...interface{})  { logf(INFO, module, format, v...) }
func Warnf(module, format string, v ...interface{})  { logf(WARN, module, format, v...) }
func Errorf(module, format string, v ...interface{}) { logf(ERROR, module, format, v...) }

