// Package game provides welcome deck orchestration functionality.
package game

import (
	"fmt"
	"time"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/simulation"
)

// Logger interface for orchestrator logging
type Logger interface {
	LogMeta(format string, args ...interface{})
	LogGame(format string, args ...interface{})
}

// WelcomeOrchestrator manages systematic welcome deck comparisons.
type WelcomeOrchestrator struct {
	deckManager    *deck.WelcomeDeckManager
	cardDB         *card.CardDB
	results        *simulation.EnhancedResults
	gameFactory    GameFactory
	config         OrchestratorConfig
	logger         Logger
}

// GameFactory interface for creating games.
type GameFactory interface {
	CreateGame(cardDB *card.CardDB) Game
}

// Game interface for running games.
type Game interface {
	AddPlayerWithDeck(deck deck.Deck) error
	Start() (winner, loser GamePlayer)
}

// GamePlayer interface for game players.
type GamePlayer interface {
	GetName() string
	GetLifeTotal() int
}

// OrchestratorConfig contains configuration for the orchestrator.
type OrchestratorConfig struct {
	GamesPerMatchup         int                           // Number of games per deck matchup
	SideboardMode          deck.SideboardIntegrationMode // How to handle sideboards
	MaxConcurrentGames     int                           // Maximum concurrent games
	EnableAbilityValidation bool                          // Whether to validate abilities
	LogLevel               string                        // Logging level
}

// DefaultOrchestratorConfig returns default configuration.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		GamesPerMatchup:         10,
		SideboardMode:          deck.SideboardIgnore,
		MaxConcurrentGames:     1,
		EnableAbilityValidation: true,
		LogLevel:               "CARD",
	}
}

// NewWelcomeOrchestrator creates a new welcome deck orchestrator.
func NewWelcomeOrchestrator(deckManager *deck.WelcomeDeckManager, cardDB *card.CardDB, gameFactory GameFactory, logger Logger) *WelcomeOrchestrator {
	return &WelcomeOrchestrator{
		deckManager: deckManager,
		cardDB:      cardDB,
		results:     simulation.NewEnhancedResults(),
		gameFactory: gameFactory,
		config:      DefaultOrchestratorConfig(),
		logger:      logger,
	}
}

// SetConfig sets the orchestrator configuration.
func (wo *WelcomeOrchestrator) SetConfig(config OrchestratorConfig) {
	wo.config = config
}

// RunSystematicComparison runs systematic comparisons between all deck pairs.
func (wo *WelcomeOrchestrator) RunSystematicComparison() error {
	wo.logger.LogMeta("Starting systematic welcome deck comparison")

	deckNames := wo.deckManager.GetAllDeckNames()
	if len(deckNames) < 2 {
		return fmt.Errorf("need at least 2 decks for comparison, found %d", len(deckNames))
	}

	wo.logger.LogMeta("Running comparisons between %d decks (%d total matchups)",
		len(deckNames), len(deckNames)*(len(deckNames)-1)/2)

	startTime := time.Now()
	totalGames := 0

	// Run all possible deck matchups
	for i := 0; i < len(deckNames); i++ {
		for j := i + 1; j < len(deckNames); j++ {
			deck1Name := deckNames[i]
			deck2Name := deckNames[j]
			
			wo.logger.LogGame("Running matchup: %s vs %s (%d games)",
				deck1Name, deck2Name, wo.config.GamesPerMatchup)

			matchupResults, err := wo.runMatchup(deck1Name, deck2Name)
			if err != nil {
				wo.logger.LogGame("Failed to run matchup %s vs %s: %v", deck1Name, deck2Name, err)
				continue
			}

			wo.results.AddMatchupResults(matchupResults)
			totalGames += wo.config.GamesPerMatchup
		}
	}

	duration := time.Since(startTime)
	wo.logger.LogMeta("Completed systematic comparison: %d games in %v (%.2f games/sec)",
		totalGames, duration, float64(totalGames)/duration.Seconds())

	return nil
}

// RunRandomizedGames runs a specified number of randomized games.
func (wo *WelcomeOrchestrator) RunRandomizedGames(numGames int) error {
	wo.logger.LogMeta("Starting randomized welcome deck games: %d games", numGames)

	startTime := time.Now()
	successfulGames := 0

	for i := 0; i < numGames; i++ {
		// Get random deck pair
		deck1, deck2, err := wo.deckManager.GetRandomDeckPair(wo.config.SideboardMode)
		if err != nil {
			wo.logger.LogGame("Failed to get random deck pair for game %d: %v", i+1, err)
			continue
		}

		// Run single game
		gameResult, err := wo.runSingleGame(deck1, deck2)
		if err != nil {
			wo.logger.LogGame("Failed to run game %d (%s vs %s): %v", i+1, deck1.Name, deck2.Name, err)
			continue
		}

		wo.results.AddGameResult(gameResult)
		successfulGames++

		if (i+1)%100 == 0 {
			wo.logger.LogMeta("Completed %d/%d randomized games", i+1, numGames)
		}
	}

	duration := time.Since(startTime)
	wo.logger.LogMeta("Completed randomized games: %d/%d successful in %v (%.2f games/sec)",
		successfulGames, numGames, duration, float64(successfulGames)/duration.Seconds())

	return nil
}

// runMatchup runs multiple games between two specific decks.
func (wo *WelcomeOrchestrator) runMatchup(deck1Name, deck2Name string) (simulation.MatchupResult, error) {
	matchup := simulation.MatchupResult{
		Deck1Name: deck1Name,
		Deck2Name: deck2Name,
		Games:     make([]simulation.GameResult, 0, wo.config.GamesPerMatchup),
	}

	for i := 0; i < wo.config.GamesPerMatchup; i++ {
		// Get fresh deck instances
		deck1, err := wo.deckManager.GetDeckByName(deck1Name, wo.config.SideboardMode)
		if err != nil {
			return matchup, fmt.Errorf("failed to get deck %s: %v", deck1Name, err)
		}

		deck2, err := wo.deckManager.GetDeckByName(deck2Name, wo.config.SideboardMode)
		if err != nil {
			return matchup, fmt.Errorf("failed to get deck %s: %v", deck2Name, err)
		}

		// Run game
		gameResult, err := wo.runSingleGame(deck1, deck2)
		if err != nil {
			wo.logger.LogGame("Failed game %d in matchup %s vs %s: %v", i+1, deck1Name, deck2Name, err)
			continue
		}

		matchup.Games = append(matchup.Games, gameResult)
	}

	// Calculate matchup statistics
	matchup.CalculateStats()
	
	return matchup, nil
}

// runSingleGame runs a single game between two decks.
func (wo *WelcomeOrchestrator) runSingleGame(deck1, deck2 deck.Deck) (simulation.GameResult, error) {
	startTime := time.Now()

	// Create game
	game := wo.gameFactory.CreateGame(wo.cardDB)

	// Add players
	if err := game.AddPlayerWithDeck(deck1); err != nil {
		return simulation.GameResult{}, fmt.Errorf("failed to add player 1: %v", err)
	}

	if err := game.AddPlayerWithDeck(deck2); err != nil {
		return simulation.GameResult{}, fmt.Errorf("failed to add player 2: %v", err)
	}

	// Run game
	winner, loser := game.Start()
	if winner == nil || loser == nil {
		return simulation.GameResult{}, fmt.Errorf("game ended without clear winner/loser")
	}

	duration := time.Since(startTime)

	result := simulation.GameResult{
		Deck1Name:        deck1.Name,
		Deck2Name:        deck2.Name,
		WinnerName:       winner.GetName(),
		LoserName:        loser.GetName(),
		WinnerFinalLife:  winner.GetLifeTotal(),
		LoserFinalLife:   loser.GetLifeTotal(),
		GameDuration:     duration,
		Timestamp:        startTime,
	}

	return result, nil
}

// GetResults returns the current results.
func (wo *WelcomeOrchestrator) GetResults() *simulation.EnhancedResults {
	return wo.results
}

// PrintSummary prints a summary of all results.
func (wo *WelcomeOrchestrator) PrintSummary() {
	wo.results.PrintSummary()
}

// PrintDetailedAnalysis prints detailed analysis of deck performance.
func (wo *WelcomeOrchestrator) PrintDetailedAnalysis() {
	wo.results.PrintDetailedAnalysis()
}

// GetTopPerformingDecks returns the top performing decks by win rate.
func (wo *WelcomeOrchestrator) GetTopPerformingDecks(limit int) []simulation.DeckPerformance {
	return wo.results.GetTopPerformingDecks(limit)
}

// ValidateAllAbilities validates abilities for all cards in all welcome decks.
func (wo *WelcomeOrchestrator) ValidateAllAbilities() error {
	if !wo.config.EnableAbilityValidation {
		wo.logger.LogMeta("Ability validation disabled")
		return nil
	}

	wo.logger.LogMeta("Starting comprehensive ability validation for welcome decks")

	// This will be implemented when we create the ability validator
	// For now, just log that validation would happen here
	wo.logger.LogMeta("Ability validation not yet implemented - placeholder")

	return nil
}

// Reset clears all results and prepares for a new run.
func (wo *WelcomeOrchestrator) Reset() {
	wo.results = simulation.NewEnhancedResults()
	wo.logger.LogMeta("Orchestrator results reset")
}
