// Package simulation provides simulation results tracking for MTG games.
package simulation

import (
	"sort"

	"github.com/mtgsim/mtgsim/internal/logger"
)

// Result represents a deck's performance with wins and losses.
type Result struct {
	Name   string
	Wins   int
	Losses int
}

// Results tracks all deck performance results.
type Results struct {
	results []Result
}

// NewResults creates a new results tracker.
func NewResults() *Results {
	return &Results{
		results: make([]Result, 0),
	}
}

// AddWin adds a win to the specified deck.
func (r *Results) AddWin(deckName string) {
	for i, deck := range r.results {
		if deck.Name == deckName {
			r.results[i].Wins++
			return
		}
	}
	r.results = append(r.results, Result{Name: deckName, Wins: 1})
}

// AddLoss adds a loss to the specified deck.
func (r *Results) AddLoss(deckName string) {
	for i, deck := range r.results {
		if deck.Name == deckName {
			r.results[i].Losses++
			return
		}
	}
	r.results = append(r.results, Result{Name: deckName, Losses: 1})
}

// SortByWinPercentage sorts the results by win percentage in descending order.
func (r *Results) SortByWinPercentage() {
	sort.Slice(r.results, func(i, j int) bool {
		winPercentageI := float64(r.results[i].Wins) / float64(r.results[i].Wins+r.results[i].Losses)
		winPercentageJ := float64(r.results[j].Wins) / float64(r.results[j].Wins+r.results[j].Losses)
		return winPercentageI > winPercentageJ
	})
}

// PrintTopResults prints the top performing decks.
func (r *Results) PrintTopResults() {
	r.SortByWinPercentage()
	for _, result := range r.results {
		if result.Wins+result.Losses > 0 {
			winPercent := float64(result.Wins) / float64(result.Wins+result.Losses) * 100
			logger.LogMeta("Deck: %s Wins: %d, Losses: %d, Win Rate: %.2f%%", 
				result.Name, result.Wins, result.Losses, winPercent)
		}
	}
}

// GetResults returns a copy of all results.
func (r *Results) GetResults() []Result {
	resultsCopy := make([]Result, len(r.results))
	copy(resultsCopy, r.results)
	return resultsCopy
}

// Clear resets all results.
func (r *Results) Clear() {
	r.results = r.results[:0]
}

// GetDeckResult returns the result for a specific deck.
func (r *Results) GetDeckResult(deckName string) (Result, bool) {
	for _, result := range r.results {
		if result.Name == deckName {
			return result, true
		}
	}
	return Result{}, false
}

// WinPercentage calculates the win percentage for a result.
func (r Result) WinPercentage() float64 {
	total := r.Wins + r.Losses
	if total == 0 {
		return 0.0
	}
	return float64(r.Wins) / float64(total) * 100
}
