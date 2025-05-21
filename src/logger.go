package main

import (
	"log"
	"os"
)

type LogLevel int

const (
	META LogLevel = iota
	GAME
	PLAYER
	CARD
)

var currentLogLevel = GAME

var logger = &Logger{
	// logger: log.New(os.Stdout, "", log.Ldate|log.Ltime), // Keep for long running tests
	logger: log.New(os.Stdout, "", log.Ltime), // Short file name and time
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
