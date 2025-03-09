package main

import (
	"fmt"
	"log"
	"os"
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
)

var logLevel = INFO

func SetLogLevel(level int) {
	logLevel = level
}

func colorize(level int, v ...interface{}) string {
	var color string
	switch level {
	case DEBUG:
		color = "\033[36m" // Cyan
	case INFO:
		color = "\033[32m" // Green
	case WARN:
		color = "\033[33m" // Yellow
	case ERROR:
		color = "\033[31m" // Red
	default:
		color = "\033[0m" // Reset
	}
	return fmt.Sprintf("%s%s\033[0m", color, fmt.Sprint(v...))
}

func Debug(v ...interface{}) {
	if logLevel <= DEBUG {
		log.Println(colorize(DEBUG, v...))
	}
}

func Info(v ...interface{}) {
	if logLevel <= INFO {
		log.Println(colorize(INFO, v...))
	}
}

func Warn(v ...interface{}) {
	if logLevel <= WARN {
		log.Println(colorize(WARN, v...))
	}
}

func Error(v ...interface{}) {
	if logLevel <= ERROR {
		log.Println(colorize(ERROR, v...))
	}
}

func init() {
	log.SetFlags(0) // Disable timestamps
	log.SetOutput(os.Stdout)
}
