package main

import (
	"log"
	"os"
)

var currentLogLevel = GAME

var logger = &Logger{
	logger: log.New(os.Stdout, "", log.Ltime),
}

type Logger struct {
	logger *log.Logger
}

func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

func LogGame(message string, args ...interface{}) {
	if currentLogLevel >= GAME {
		logger.logger.Printf("GAME: "+message, args...)
	}
}

func LogPlayer(message string, args ...interface{}) {
	if currentLogLevel >= PLAYER {
		logger.logger.Printf("PLAYER: "+message, args...)
	}
}

func LogCard(message string, args ...interface{}) {
	if currentLogLevel >= CARD {
		logger.logger.Printf("CARD: "+message, args...)
	}
}

func LogDeck(message string, args ...interface{}) {
	if currentLogLevel >= CARD {
		logger.logger.Printf("DECK: "+message, args...)
	}
}

func LogMeta(message string, args ...interface{}) {
	if currentLogLevel >= META {
		logger.logger.Printf("META: "+message, args...)
	}
}
