// Package logger provides logging functionality for MTG simulation.
package logger

import (
	"log"
	"os"

	"github.com/mtgsim/mtgsim/pkg/game"
)

var currentLogLevel = game.GAME

var logger = &Logger{
	logger: log.New(os.Stdout, "", log.Ltime),
}

// Logger wraps the standard logger with MTG-specific functionality.
type Logger struct {
	logger *log.Logger
}

// SetLogLevel sets the current logging level.
func SetLogLevel(level game.LogLevel) {
	currentLogLevel = level
}

// LogGame logs game-level messages.
func LogGame(message string, args ...interface{}) {
	if currentLogLevel >= game.GAME {
		logger.logger.Printf("GAME: "+message, args...)
	}
}

// LogPlayer logs player-level messages.
func LogPlayer(message string, args ...interface{}) {
	if currentLogLevel >= game.PLAYER {
		logger.logger.Printf("PLAYER: "+message, args...)
	}
}

// LogCard logs card-level messages.
func LogCard(message string, args ...interface{}) {
	if currentLogLevel >= game.CARD {
		logger.logger.Printf("CARD: "+message, args...)
	}
}

// LogDeck logs deck-level messages.
func LogDeck(message string, args ...interface{}) {
	if currentLogLevel >= game.CARD {
		logger.logger.Printf("DECK: "+message, args...)
	}
}

// LogMeta logs meta-level messages.
func LogMeta(message string, args ...interface{}) {
	if currentLogLevel >= game.META {
		logger.logger.Printf("META: "+message, args...)
	}
}

// ParseLogLevel parses a string into a LogLevel.
func ParseLogLevel(level string) game.LogLevel {
	switch level {
	case "META":
		return game.META
	case "GAME":
		return game.GAME
	case "PLAYER":
		return game.PLAYER
	case "CARD":
		return game.CARD
	default:
		return game.CARD
	}
}
