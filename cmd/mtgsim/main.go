// MTGSim - Magic: The Gathering deck simulation tool
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/simulation"
)

func main() {
	// Define command-line flags
	numGames := flag.Int("games", 1, "Number of games to simulate")
	deckDir := flag.String("decks", "decks/1v1", "Directory containing deck files")
	logLevel := flag.String("log", "CARD", "Log level (META, GAME, PLAYER, CARD)")

	// Parse the flags
	flag.Parse()

	// Set the log level
	logger.SetLogLevel(logger.ParseLogLevel(*logLevel))

	// Load card database
	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Printf("Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

	// Get the decks
	decks, err := simulation.GetDecks(*deckDir)
	if err != nil || len(decks) == 0 {
		fmt.Println("Error: No decks found in the specified directory.")
		os.Exit(1)
	}
	logger.LogMeta("Found %d deck files", len(decks))

	// Initialize results tracker
	results := simulation.NewResults()

	start := time.Now() // Start timing

	// Simulate games
	logger.LogMeta("Starting simulation of %d games...", *numGames)
	for i := 0; i < *numGames; i++ {
		deck1 := simulation.GetRandom(decks)
		deck2 := simulation.GetRandom(decks)
		// Ensure two different decks
		for deck2 == deck1 && len(decks) > 1 {
			deck2 = simulation.GetRandom(decks)
		}

		// Create and run game
		g := NewGame(cardDB)
		err := g.AddPlayer(deck1)
		if err != nil {
			logger.LogGame("Error adding player 1: %v", err)
			continue
		}
		err = g.AddPlayer(deck2)
		if err != nil {
			logger.LogGame("Error adding player 2: %v", err)
			continue
		}

		winner, loser := g.Start()
		if winner != nil && loser != nil {
			results.AddWin(winner.Name)
			results.AddLoss(loser.Name)
		}
	}

	elapsed := time.Since(start) // End timing

	// Calculate average games per second
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
	results.PrintTopResults()
	fmt.Printf("Simulated %d games in %.2fs: %d games/sec\n", *numGames, elapsed.Seconds(), gamesPerSecond)
	logger.LogMeta("Simulation completed.")
}
