package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func sliceGet[T any](slice []T, index int) (T, []T) {
	var out T
	if index < 0 || index >= len(slice) {
		return out, slice
	}
	if len(slice) == 0 {
		return out, slice
	}

	out = slice[index]
	if index == len(slice)-1 {
		slice = slice[:index]
		return out, slice
	}
	slice = append(slice[:index], slice[index+1:]...)
	return out, slice
}

func getDecks(dir string) ([]string, error) {
	var fileList []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			subDirFiles, err := getDecks(dir + "/" + file.Name())
			if err != nil {
				return nil, err
			}
			fileList = append(fileList, subDirFiles...)
		} else {
			fileList = append(fileList, dir+"/"+file.Name())
		}
	}
	return fileList, nil
}

func getRandom[T any](slice []T) T {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return slice[r.Intn(len(slice))]
}

func parseLogLevel(level string) LogLevel {
	switch level {
	case "META":
		return META
	case "GAME":
		return GAME
	case "PLAYER":
		return PLAYER
	case "CARD":
		return CARD
	default:
		fmt.Printf("Unknown log level '%s', defaulting to CARD\n", level)
		return CARD
	}
}

func main() {
	// Define command-line flags
	numGames := flag.Int("games", 1, "Number of games to simulate")
	deckDir := flag.String("decks", "../decks/1v1", "Directory containing deck files")
	logLevel := flag.String("log", "CARD", "Log level (META, GAME, PLAYER, CARD)")

	// Parse the flags
	flag.Parse()

	// Set the log level
	SetLogLevel(parseLogLevel(*logLevel))

	// Get the decks
	decks, err := getDecks(*deckDir)
	if err != nil || len(decks) == 0 {
		fmt.Println("Error: No decks found in the specified directory.")
		return
	}

	start := time.Now() // Start timing

	// Simulate games
	for i := 0; i < *numGames; i++ {
		g := newGame()
		g.AddPlayer(getRandom(decks))
		g.AddPlayer(getRandom(decks))
		g.Start()
		AddWin(g.winner.Name)
		AddLoss(g.loser.Name)
	}

	elapsed := time.Since(start) // End timing

	// Calculate average games per second (truncate to int)
	gamesPerSecond := int(float64(*numGames) / elapsed.Seconds())

	// Log to file
	logFile, err := os.OpenFile("simulation.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer func() {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Error closing log file: %v\n", err)
			}
		}()
		if _, err := fmt.Fprintf(logFile, "Simulated %d games in %.2fs: %d games/sec\n", *numGames, elapsed.Seconds(), gamesPerSecond); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to log file: %v\n", err)
		}
	}

	// Print results
	PrintTopWinners()
	fmt.Printf("Simulated %d games in %.2fs: %d games/sec\n", *numGames, elapsed.Seconds(), gamesPerSecond)
	LogMeta("Simulation completed.")
}
