// Package logger provides logging functionality for MTG simulation.
package logger

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mtgsim/mtgsim/pkg/types"
)

var currentLogLevel = types.GAME

var logger = &Logger{
	logger: log.New(os.Stdout, "", log.Ltime),
}

// Logger wraps the standard logger with MTG-specific functionality.
type Logger struct {
	logger *log.Logger
}

// SetLogLevel sets the current logging level.
func SetLogLevel(level types.LogLevel) {
	currentLogLevel = level
}

// LogGame logs game-level messages.
func LogGame(message string, args ...interface{}) {
	if currentLogLevel >= types.GAME {
		logger.logger.Printf("GAME: "+message, args...)
	}
}

// LogPlayer logs player-level messages.
func LogPlayer(message string, args ...interface{}) {
	if currentLogLevel >= types.PLAYER {
		logger.logger.Printf("PLAYER: "+message, args...)
	}
}

// LogCard logs card-level messages.
func LogCard(message string, args ...interface{}) {
	if currentLogLevel >= types.CARD {
		logger.logger.Printf("CARD: "+message, args...)
	}
}

// LogDeck logs deck-level messages.
func LogDeck(message string, args ...interface{}) {
	if currentLogLevel >= types.CARD {
		logger.logger.Printf("DECK: "+message, args...)
	}
}

// LogMeta logs meta-level messages.
func LogMeta(message string, args ...interface{}) {
	if currentLogLevel >= types.META {
		logger.logger.Printf("META: "+message, args...)
	}
}

// ParseLogLevel parses a string into a LogLevel.
func ParseLogLevel(level string) types.LogLevel {
	switch level {
	case "META":
		return types.META
	case "GAME":
		return types.GAME
	case "PLAYER":
		return types.PLAYER
	case "CARD":
		return types.CARD
	default:
		return types.CARD
	}
}

// ParsingFailureLogger handles logging of card parsing failures
type ParsingFailureLogger struct {
	logFile string
	cache   map[string]bool // Cache to avoid duplicate entries
}

var parsingLogger *ParsingFailureLogger

// InitParsingLogger initializes the parsing failure logger
func InitParsingLogger() error {
	if parsingLogger != nil {
		return nil // Already initialized
	}

	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	logFile := filepath.Join(logsDir, "parsing_failures.log")
	parsingLogger = &ParsingFailureLogger{
		logFile: logFile,
		cache:   make(map[string]bool),
	}

	// Load existing entries to avoid duplicates
	if err := parsingLogger.loadExistingEntries(); err != nil {
		LogCard("Warning: Failed to load existing parsing failure entries: %v", err)
	}

	return nil
}

// loadExistingEntries loads existing log entries to avoid duplicates
func (pfl *ParsingFailureLogger) loadExistingEntries() error {
	file, err := os.Open(pfl.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's fine
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Extract card name from log line (format: "TIMESTAMP [CARD_NAME] ...")
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			start := strings.Index(line, "[") + 1
			end := strings.Index(line, "]")
			if start < end {
				cardName := line[start:end]
				pfl.cache[cardName] = true
			}
		}
	}

	return scanner.Err()
}

// LogParsingFailure logs a card parsing failure if not already logged
func LogParsingFailure(cardName, oracleText, errorDetails string) {
	if parsingLogger == nil {
		if err := InitParsingLogger(); err != nil {
			LogCard("Failed to initialize parsing logger: %v", err)
			return
		}
	}

	// Check if already logged
	if parsingLogger.cache[cardName] {
		return // Already logged, skip
	}

	// Mark as logged
	parsingLogger.cache[cardName] = true

	// Create log entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("%s [%s] Parsing failed\nOracle Text: %s\nError: %s\n---\n",
		timestamp, cardName, oracleText, errorDetails)

	// Append to log file
	file, err := os.OpenFile(parsingLogger.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		LogCard("Failed to open parsing failure log: %v", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(logEntry); err != nil {
		LogCard("Failed to write parsing failure log: %v", err)
	}

	// Also log to console for immediate visibility
	LogCard("Parsing failure logged for card: %s", cardName)
}
