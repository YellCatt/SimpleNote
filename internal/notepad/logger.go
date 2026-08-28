package notepad

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	appLog   *log.Logger
	logLevel string
)

func InitLogger(baseDir string) error {
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, time.Now().Format("2006-01-02")+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	appLog = log.New(io.MultiWriter(logFile, os.Stdout), "", 0)
	return nil
}

func SetLogLevel(level string) {
	logLevel = normalizeLogLevel(level)
	if logLevel == "" {
		logLevel = defaultLogLevel
	}
}

func logInfo(format string, args ...any) {
	if appLog == nil {
		return
	}
	if !shouldLog(logLevelInfo) {
		return
	}
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	appLog.Printf("[INFO]  %s "+format, append([]any{timestamp}, args...)...)
}

func logError(format string, args ...any) {
	if appLog == nil {
		return
	}
	if !shouldLog(logLevelError) {
		return
	}
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	appLog.Printf("[ERROR] %s "+format, append([]any{timestamp}, args...)...)
}

func logDebug(format string, args ...any) {
	if appLog == nil {
		return
	}
	if !shouldLog(logLevelDebug) {
		return
	}
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	appLog.Printf("[DEBUG] %s "+format, append([]any{timestamp}, args...)...)
}

func shouldLog(level string) bool {
	if logLevel == "" {
		logLevel = defaultLogLevel
	}
	switch logLevel {
	case logLevelDebug:
		return true
	case logLevelInfo:
		return level == logLevelInfo || level == logLevelError
	case logLevelError:
		return level == logLevelError
	default:
		return true
	}
}

func currentLogLevel() string {
	if logLevel == "" {
		return strings.ToUpper(defaultLogLevel)
	}
	return strings.ToUpper(logLevel)
}