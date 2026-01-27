package observability

import (
	"os"
	"strings"
	"sync"
)

// Level represents a log severity level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel Level
	levelOnce    sync.Once
)

// initLevel reads LOG_LEVEL from environment and sets the current level.
// Called automatically on first use. Valid values: debug, info, warn, error.
// Defaults to info if not set or invalid.
func initLevel() {
	levelOnce.Do(func() {
		envLevel := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
		switch envLevel {
		case "debug":
			currentLevel = LevelDebug
		case "info":
			currentLevel = LevelInfo
		case "warn", "warning":
			currentLevel = LevelWarn
		case "error":
			currentLevel = LevelError
		default:
			currentLevel = LevelInfo
		}
	})
}

// GetLevel returns the current log level
func GetLevel() Level {
	initLevel()
	return currentLevel
}

// ShouldLog returns true if messages at the given level should be logged
func ShouldLog(level Level) bool {
	initLevel()
	return level >= currentLevel
}

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
