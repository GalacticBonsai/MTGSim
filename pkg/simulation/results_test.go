package simulation

import (
	"testing"
)

func TestResults(t *testing.T) {
	results := NewResults()

	// Test adding wins and losses
	results.AddWin("Deck A")
	results.AddWin("Deck A")
	results.AddLoss("Deck A")
	
	results.AddWin("Deck B")
	results.AddLoss("Deck B")
	results.AddLoss("Deck B")

	// Test getting results
	allResults := results.GetResults()
	if len(allResults) != 2 {
		t.Errorf("Expected 2 decks, got %d", len(allResults))
	}

	// Test specific deck result
	deckA, found := results.GetDeckResult("Deck A")
	if !found {
		t.Error("Deck A not found")
	}
	if deckA.Wins != 2 || deckA.Losses != 1 {
		t.Errorf("Expected Deck A to have 2 wins and 1 loss, got %d wins and %d losses", deckA.Wins, deckA.Losses)
	}

	// Test win percentage
	winPercent := deckA.WinPercentage()
	if winPercent < 66.0 || winPercent > 67.0 {
		t.Errorf("Expected win percentage around 66.67%%, got %.2f%%", winPercent)
	}

	// Test sorting
	results.SortByWinPercentage()
	sortedResults := results.GetResults()
	if sortedResults[0].Name != "Deck A" {
		t.Error("Expected Deck A to be first after sorting by win percentage")
	}

	// Test clear
	results.Clear()
	if len(results.GetResults()) != 0 {
		t.Error("Expected results to be empty after clear")
	}
}
