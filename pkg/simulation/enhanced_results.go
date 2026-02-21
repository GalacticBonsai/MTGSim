// Package simulation provides enhanced results tracking for welcome deck analysis.
package simulation

import (
	"sort"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
)

// EnhancedResults provides comprehensive tracking of deck performance.
type EnhancedResults struct {
	gameResults    []GameResult
	matchupResults []MatchupResult
	deckStats      map[string]*DeckPerformance
}

// GameResult represents the result of a single game.
type GameResult struct {
	Deck1Name       string
	Deck2Name       string
	WinnerName      string
	LoserName       string
	WinnerFinalLife int
	LoserFinalLife  int
	GameDuration    time.Duration
	Timestamp       time.Time
	TurnsPlayed     int
	AbilitiesUsed   int
	GameEndReason   string
}

// MatchupResult represents the results of multiple games between two decks.
type MatchupResult struct {
	Deck1Name     string
	Deck2Name     string
	Games         []GameResult
	Deck1Wins     int
	Deck2Wins     int
	AverageLength time.Duration
	WinRate1      float64
	WinRate2      float64
}

// DeckPerformance tracks comprehensive performance metrics for a deck.
type DeckPerformance struct {
	Name                string
	TotalGames          int
	Wins                int
	Losses              int
	WinRate             float64
	AverageGameLength   time.Duration
	AverageWinningLife  float64
	AverageLosingLife   float64
	TotalTurnsPlayed    int
	AverageTurnsPerGame float64
	TotalAbilitiesUsed  int
	AbilitiesPerGame    float64
	Matchups            map[string]MatchupStats
}

// MatchupStats tracks performance against specific opponents.
type MatchupStats struct {
	OpponentName string
	Games        int
	Wins         int
	Losses       int
	WinRate      float64
}

// NewEnhancedResults creates a new enhanced results tracker.
func NewEnhancedResults() *EnhancedResults {
	return &EnhancedResults{
		gameResults:    make([]GameResult, 0),
		matchupResults: make([]MatchupResult, 0),
		deckStats:      make(map[string]*DeckPerformance),
	}
}

// AddGameResult adds a single game result.
func (er *EnhancedResults) AddGameResult(result GameResult) {
	er.gameResults = append(er.gameResults, result)
	er.updateDeckStats(result)
}

// AddMatchupResults adds results from a complete matchup.
func (er *EnhancedResults) AddMatchupResults(matchup MatchupResult) {
	er.matchupResults = append(er.matchupResults, matchup)
	
	// Add individual game results
	for _, game := range matchup.Games {
		er.gameResults = append(er.gameResults, game)
		er.updateDeckStats(game)
	}
}

// updateDeckStats updates deck performance statistics.
func (er *EnhancedResults) updateDeckStats(result GameResult) {
	// Update winner stats
	if er.deckStats[result.WinnerName] == nil {
		er.deckStats[result.WinnerName] = &DeckPerformance{
			Name:     result.WinnerName,
			Matchups: make(map[string]MatchupStats),
		}
	}
	
	winnerStats := er.deckStats[result.WinnerName]
	winnerStats.TotalGames++
	winnerStats.Wins++
	winnerStats.TotalTurnsPlayed += result.TurnsPlayed
	winnerStats.TotalAbilitiesUsed += result.AbilitiesUsed
	
	// Update loser stats
	if er.deckStats[result.LoserName] == nil {
		er.deckStats[result.LoserName] = &DeckPerformance{
			Name:     result.LoserName,
			Matchups: make(map[string]MatchupStats),
		}
	}
	
	loserStats := er.deckStats[result.LoserName]
	loserStats.TotalGames++
	loserStats.Losses++
	loserStats.TotalTurnsPlayed += result.TurnsPlayed
	loserStats.TotalAbilitiesUsed += result.AbilitiesUsed
	
	// Update matchup stats
	er.updateMatchupStats(result.WinnerName, result.LoserName, true)
	er.updateMatchupStats(result.LoserName, result.WinnerName, false)
	
	// Recalculate derived stats
	er.calculateDerivedStats(winnerStats)
	er.calculateDerivedStats(loserStats)
}

// updateMatchupStats updates head-to-head matchup statistics.
func (er *EnhancedResults) updateMatchupStats(deckName, opponentName string, won bool) {
	deckStats := er.deckStats[deckName]
	
	if deckStats.Matchups[opponentName].OpponentName == "" {
		deckStats.Matchups[opponentName] = MatchupStats{
			OpponentName: opponentName,
		}
	}
	
	matchup := deckStats.Matchups[opponentName]
	matchup.Games++
	if won {
		matchup.Wins++
	} else {
		matchup.Losses++
	}
	matchup.WinRate = float64(matchup.Wins) / float64(matchup.Games) * 100
	
	deckStats.Matchups[opponentName] = matchup
}

// calculateDerivedStats calculates derived statistics for a deck.
func (er *EnhancedResults) calculateDerivedStats(stats *DeckPerformance) {
	if stats.TotalGames == 0 {
		return
	}
	
	stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames) * 100
	stats.AverageTurnsPerGame = float64(stats.TotalTurnsPlayed) / float64(stats.TotalGames)
	stats.AbilitiesPerGame = float64(stats.TotalAbilitiesUsed) / float64(stats.TotalGames)
	
	// Calculate average game length and life totals
	var totalDuration time.Duration
	var totalWinningLife, totalLosingLife float64
	var winningGames, losingGames int
	
	for _, result := range er.gameResults {
		if result.WinnerName == stats.Name {
			totalDuration += result.GameDuration
			totalWinningLife += float64(result.WinnerFinalLife)
			winningGames++
		} else if result.LoserName == stats.Name {
			totalDuration += result.GameDuration
			totalLosingLife += float64(result.LoserFinalLife)
			losingGames++
		}
	}
	
	if stats.TotalGames > 0 {
		stats.AverageGameLength = totalDuration / time.Duration(stats.TotalGames)
	}
	if winningGames > 0 {
		stats.AverageWinningLife = totalWinningLife / float64(winningGames)
	}
	if losingGames > 0 {
		stats.AverageLosingLife = totalLosingLife / float64(losingGames)
	}
}

// CalculateStats calculates statistics for a matchup result.
func (mr *MatchupResult) CalculateStats() {
	mr.Deck1Wins = 0
	mr.Deck2Wins = 0
	var totalDuration time.Duration
	
	for _, game := range mr.Games {
		totalDuration += game.GameDuration
		if game.WinnerName == mr.Deck1Name {
			mr.Deck1Wins++
		} else if game.WinnerName == mr.Deck2Name {
			mr.Deck2Wins++
		}
	}
	
	totalGames := len(mr.Games)
	if totalGames > 0 {
		mr.WinRate1 = float64(mr.Deck1Wins) / float64(totalGames) * 100
		mr.WinRate2 = float64(mr.Deck2Wins) / float64(totalGames) * 100
		mr.AverageLength = totalDuration / time.Duration(totalGames)
	}
}

// GetTopPerformingDecks returns the top performing decks by win rate.
func (er *EnhancedResults) GetTopPerformingDecks(limit int) []DeckPerformance {
	decks := make([]DeckPerformance, 0, len(er.deckStats))
	
	for _, stats := range er.deckStats {
		decks = append(decks, *stats)
	}
	
	// Sort by win rate (descending), then by total games (descending)
	sort.Slice(decks, func(i, j int) bool {
		if decks[i].WinRate == decks[j].WinRate {
			return decks[i].TotalGames > decks[j].TotalGames
		}
		return decks[i].WinRate > decks[j].WinRate
	})
	
	if limit > 0 && limit < len(decks) {
		decks = decks[:limit]
	}
	
	return decks
}

// PrintSummary prints a summary of all results.
func (er *EnhancedResults) PrintSummary() {
	logger.LogMeta("=== WELCOME DECK PERFORMANCE SUMMARY ===")
	logger.LogMeta("Total games played: %d", len(er.gameResults))
	logger.LogMeta("Total matchups completed: %d", len(er.matchupResults))
	logger.LogMeta("Decks analyzed: %d", len(er.deckStats))
	
	if len(er.deckStats) == 0 {
		logger.LogMeta("No results to display")
		return
	}
	
	// Show top performing decks
	topDecks := er.GetTopPerformingDecks(5)
	logger.LogMeta("\nTop 5 Performing Decks:")
	for i, deck := range topDecks {
		logger.LogMeta("%d. %s: %.1f%% win rate (%d-%d, %d games)",
			i+1, deck.Name, deck.WinRate, deck.Wins, deck.Losses, deck.TotalGames)
	}
}

// PrintDetailedAnalysis prints detailed analysis of deck performance.
func (er *EnhancedResults) PrintDetailedAnalysis() {
	logger.LogMeta("=== DETAILED DECK ANALYSIS ===")
	
	decks := er.GetTopPerformingDecks(0) // Get all decks, sorted
	
	for _, deck := range decks {
		logger.LogMeta("\n--- %s ---", deck.Name)
		logger.LogMeta("Overall: %d-%d (%.1f%% win rate)", deck.Wins, deck.Losses, deck.WinRate)
		logger.LogMeta("Average game length: %v", deck.AverageGameLength)
		logger.LogMeta("Average turns per game: %.1f", deck.AverageTurnsPerGame)
		logger.LogMeta("Abilities per game: %.1f", deck.AbilitiesPerGame)
		
		if deck.Wins > 0 {
			logger.LogMeta("Average life when winning: %.1f", deck.AverageWinningLife)
		}
		if deck.Losses > 0 {
			logger.LogMeta("Average life when losing: %.1f", deck.AverageLosingLife)
		}
		
		// Show matchup details
		if len(deck.Matchups) > 0 {
			logger.LogMeta("Head-to-head records:")
			for _, matchup := range deck.Matchups {
				logger.LogMeta("  vs %s: %d-%d (%.1f%%)", 
					matchup.OpponentName, matchup.Wins, matchup.Losses, matchup.WinRate)
			}
		}
	}
}

// GetTotalGames returns the total number of games played.
func (er *EnhancedResults) GetTotalGames() int {
	return len(er.gameResults)
}

// GetDeckStats returns performance stats for a specific deck.
func (er *EnhancedResults) GetDeckStats(deckName string) (*DeckPerformance, bool) {
	stats, exists := er.deckStats[deckName]
	return stats, exists
}

// Clear resets all results.
func (er *EnhancedResults) Clear() {
	er.gameResults = er.gameResults[:0]
	er.matchupResults = er.matchupResults[:0]
	er.deckStats = make(map[string]*DeckPerformance)
}
