// MTGSim - Magic: The Gathering deck simulation tool
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/simulation"
)

func main() {
	// Define command-line flags
	numGames := flag.Int("games", 1, "Number of games to simulate")
	deckDir := flag.String("decks", "decks/1v1", "Directory containing deck files")
	logLevel := flag.String("log", "CARD", "Log level (META, GAME, PLAYER, CARD)")

	// Welcome deck specific flags
	welcomeMode := flag.Bool("welcome", false, "Use welcome deck mode")
	welcomeDir := flag.String("welcome-dir", "decks/welcome", "Directory containing welcome deck files")
	systematic := flag.Bool("systematic", false, "Run systematic comparison between all deck pairs (welcome mode only)")
	gamesPerMatchup := flag.Int("matchup-games", 10, "Number of games per matchup in systematic mode")
	validateAbilities := flag.Bool("validate", true, "Validate card abilities (welcome mode only)")
	sideboardMode := flag.String("sideboard", "ignore", "Sideboard mode: ignore, add, replace (welcome mode only)")

	// Parse the flags
	flag.Parse()

	// Set the log level
	logger.SetLogLevel(logger.ParseLogLevel(*logLevel))

	// Initialize parsing failure logger
	if err := logger.InitParsingLogger(); err != nil {
		fmt.Printf("Warning: Failed to initialize parsing logger: %v\n", err)
	}

	// Load card database
	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Printf("Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

	// Check if welcome mode is enabled
	if *welcomeMode {
		runWelcomeMode(cardDB, *welcomeDir, *numGames, *systematic, *gamesPerMatchup, *validateAbilities, *sideboardMode)
		return
	}

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

// LoggerAdapter adapts the global logger to the game.Logger interface
type LoggerAdapter struct{}

func (la *LoggerAdapter) LogMeta(format string, args ...interface{}) {
	logger.LogMeta(format, args...)
}

func (la *LoggerAdapter) LogGame(format string, args ...interface{}) {
	logger.LogGame(format, args...)
}

// WelcomeGameFactory creates simple games for welcome deck testing.
type WelcomeGameFactory struct{}

// CreateGame creates a new simple game instance.
func (wgf *WelcomeGameFactory) CreateGame(cardDB *card.CardDB) game.Game {
	return &WelcomeGameAdapter{
		cardDB:  cardDB,
		players: make([]*WelcomePlayer, 0, 2),
	}
}

// WelcomeGameAdapter provides a simple game implementation for testing.
type WelcomeGameAdapter struct {
	cardDB  *card.CardDB
	players []*WelcomePlayer
}

// WelcomePlayer represents a simple player for testing.
type WelcomePlayer struct {
	name      string
	deck      deck.Deck
	lifeTotal int
}

// GetName returns the player's name.
func (wp *WelcomePlayer) GetName() string {
	return wp.name
}

// GetLifeTotal returns the player's life total.
func (wp *WelcomePlayer) GetLifeTotal() int {
	return wp.lifeTotal
}

// AddPlayerWithDeck adds a player with a specific deck to the game.
func (wga *WelcomeGameAdapter) AddPlayerWithDeck(playerDeck deck.Deck) error {
	if len(wga.players) >= 2 {
		return fmt.Errorf("game already has maximum players")
	}

	player := &WelcomePlayer{
		name:      playerDeck.Name,
		deck:      playerDeck,
		lifeTotal: 20,
	}

	wga.players = append(wga.players, player)
	return nil
}

// Start runs the game and returns winner and loser.
func (wga *WelcomeGameAdapter) Start() (winner, loser game.GamePlayer) {
	if len(wga.players) < 2 {
		return nil, nil
	}

	// Simple simulation: determine winner based on deck characteristics
	player1 := wga.players[0]
	player2 := wga.players[1]

	// Calculate deck scores based on card types and costs
	score1 := wga.calculateDeckScore(player1.deck)
	score2 := wga.calculateDeckScore(player2.deck)

	if score1 > score2 {
		player2.lifeTotal = 0 // Simulate loss
		return player1, player2
	} else {
		player1.lifeTotal = 0 // Simulate loss
		return player2, player1
	}
}

// calculateDeckScore calculates a simple score for deck strength.
func (wga *WelcomeGameAdapter) calculateDeckScore(d deck.Deck) float64 {
	score := 0.0

	for _, cardData := range d.Cards {
		// Basic scoring based on card type and CMC
		switch {
		case cardData.TypeLine == "Land":
			score += 0.5 // Lands provide consistency
		case cardData.CMC <= 2:
			score += 2.0 // Low cost cards are efficient
		case cardData.CMC <= 4:
			score += 1.5 // Mid-range cards
		default:
			score += 1.0 // High cost cards
		}

		// Bonus for cards with abilities
		if cardData.OracleText != "" {
			score += 0.5
		}
	}

	return score
}

// runWelcomeMode runs the welcome deck simulation mode
func runWelcomeMode(cardDB *card.CardDB, welcomeDir string, numGames int, systematic bool, gamesPerMatchup int, validateAbilities bool, sideboardMode string) {
	// Create welcome deck manager
	deckManager := deck.NewWelcomeDeckManager(cardDB)

	// Load welcome decks
	logger.LogMeta("Loading welcome decks from: %s", welcomeDir)
	if err := deckManager.LoadWelcomeDecks(welcomeDir); err != nil {
		fmt.Printf("Error loading welcome decks: %v\n", err)
		os.Exit(1)
	}

	// Parse sideboard mode
	var sbMode deck.SideboardIntegrationMode
	switch sideboardMode {
	case "ignore":
		sbMode = deck.SideboardIgnore
	case "add":
		sbMode = deck.SideboardAdd
	case "replace":
		sbMode = deck.SideboardReplace
	default:
		fmt.Printf("Invalid sideboard mode: %s (use ignore, add, or replace)\n", sideboardMode)
		os.Exit(1)
	}

	// Create orchestrator
	gameFactory := &WelcomeGameFactory{}
	loggerAdapter := &LoggerAdapter{}
	orchestrator := game.NewWelcomeOrchestrator(deckManager, cardDB, gameFactory, loggerAdapter)

	// Configure orchestrator
	config := game.DefaultOrchestratorConfig()
	config.GamesPerMatchup = gamesPerMatchup
	config.SideboardMode = sbMode
	config.EnableAbilityValidation = validateAbilities
	orchestrator.SetConfig(config)

	// Validate abilities if requested
	if validateAbilities {
		logger.LogMeta("Running ability validation...")
		validator := ability.NewWelcomeValidator(cardDB)
		if err := validator.ValidateWelcomeDecks(deckManager); err != nil {
			logger.LogMeta("Ability validation failed: %v", err)
		}
	}

	// Run simulation
	if systematic {
		logger.LogMeta("Running systematic comparison between all deck pairs...")
		if err := orchestrator.RunSystematicComparison(); err != nil {
			fmt.Printf("Error running systematic comparison: %v\n", err)
			os.Exit(1)
		}
	} else {
		logger.LogMeta("Running randomized games...")
		if err := orchestrator.RunRandomizedGames(numGames); err != nil {
			fmt.Printf("Error running randomized games: %v\n", err)
			os.Exit(1)
		}
	}

	// Print results
	orchestrator.PrintSummary()
	orchestrator.PrintDetailedAnalysis()

	// Show top performing decks
	logger.LogMeta("\n=== TOP PERFORMING DECKS ===")
	topDecks := orchestrator.GetTopPerformingDecks(3)
	for i, deck := range topDecks {
		logger.LogMeta("%d. %s: %.1f%% win rate (%d games)",
			i+1, deck.Name, deck.WinRate, deck.TotalGames)
	}

	logger.LogMeta("\nWelcome deck simulation completed successfully!")
}
