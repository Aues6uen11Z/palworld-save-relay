// Package logger provides a process-wide file logger used across the app for
// diagnostics: startup, config, save detection, host conversion, cloud sync,
// backups, import/export and locks. Public operations log their entry, outcome
// and any errors, so runtime issues are traceable from the log file without a
// visible console (the GUI hides its console window).
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu    sync.Mutex
	file  *os.File
	ready bool
)

// Init opens (or creates) the log file at path for appending. Must be called
// once at startup; log calls before Init are silently dropped. The parent
// directory is created if missing.
func Init(path string) error {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	file = f
	ready = true
	return nil
}

// Close flushes and closes the log file.
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	ready = false
	return err
}

// DefaultPath returns the default log file path
// (%APPDATA%/PalSaveRelay/app.log), or "" if APPDATA is unset.
func DefaultPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "PalSaveRelay", "app.log")
}

func write(level, msg string) {
	mu.Lock()
	defer mu.Unlock()
	if !ready || file == nil {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05.000") + " [" + level + "] " + msg + "\n"
	file.WriteString(line)
}

// Infof/Warnf/Errorf format and log at the given level.
func Infof(format string, args ...any)  { write("INFO", fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)  { write("WARN", fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any) { write("ERROR", fmt.Sprintf(format, args...)) }

// Info/Warn/Error log a plain message at the given level.
func Info(msg string)  { write("INFO", msg) }
func Warn(msg string)  { write("WARN", msg) }
func Error(msg string) { write("ERROR", msg) }
