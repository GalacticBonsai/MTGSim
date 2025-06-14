package main

import (
	"sort"
	"testing"
)

// WinRecord represents a player's win/loss record
type WinRecord struct {
	PlayerName string
	Wins       int
	Losses     int
	WinRate    float64
}

// WinTracker tracks wins and losses for players
type WinTracker struct {
	Records map[string]*WinRecord
}

// NewWinTracker creates a new win tracker
func NewWinTracker() *WinTracker {
	return &WinTracker{
		Records: make(map[string]*WinRecord),
	}
}

// AddWin adds a win for the specified player
func (wt *WinTracker) AddWin(playerName string) {
	if wt.Records[playerName] == nil {
		wt.Records[playerName] = &WinRecord{
			PlayerName: playerName,
			Wins:       0,
			Losses:     0,
			WinRate:    0.0,
		}
	}
	wt.Records[playerName].Wins++
	wt.updateWinRate(playerName)
}

// AddLoss adds a loss for the specified player
func (wt *WinTracker) AddLoss(playerName string) {
	if wt.Records[playerName] == nil {
		wt.Records[playerName] = &WinRecord{
			PlayerName: playerName,
			Wins:       0,
			Losses:     0,
			WinRate:    0.0,
		}
	}
	wt.Records[playerName].Losses++
	wt.updateWinRate(playerName)
}

// updateWinRate calculates and updates the win rate for a player
func (wt *WinTracker) updateWinRate(playerName string) {
	record := wt.Records[playerName]
	totalGames := record.Wins + record.Losses
	if totalGames > 0 {
		record.WinRate = float64(record.Wins) / float64(totalGames)
	} else {
		record.WinRate = 0.0
	}
}

// SortWinners returns a sorted slice of win records by win rate (descending)
func (wt *WinTracker) SortWinners() []*WinRecord {
	var records []*WinRecord
	for _, record := range wt.Records {
		records = append(records, record)
	}
	
	sort.Slice(records, func(i, j int) bool {
		// Sort by win rate descending, then by wins descending
		if records[i].WinRate == records[j].WinRate {
			return records[i].Wins > records[j].Wins
		}
		return records[i].WinRate > records[j].WinRate
	})
	
	return records
}

func TestAddWin(t *testing.T) {
	tracker := NewWinTracker()
	
	// Test adding a win to a new player
	tracker.AddWin("Player1")
	
	record := tracker.Records["Player1"]
	if record == nil {
		t.Fatal("Expected record to be created for Player1")
	}
	
	if record.Wins != 1 {
		t.Errorf("Expected 1 win, got %d", record.Wins)
	}
	
	if record.Losses != 0 {
		t.Errorf("Expected 0 losses, got %d", record.Losses)
	}
	
	if record.WinRate != 1.0 {
		t.Errorf("Expected win rate 1.0, got %f", record.WinRate)
	}
	
	// Test adding another win
	tracker.AddWin("Player1")
	
	if record.Wins != 2 {
		t.Errorf("Expected 2 wins, got %d", record.Wins)
	}
	
	if record.WinRate != 1.0 {
		t.Errorf("Expected win rate 1.0, got %f", record.WinRate)
	}
}

func TestAddLoss(t *testing.T) {
	tracker := NewWinTracker()
	
	// Test adding a loss to a new player
	tracker.AddLoss("Player1")
	
	record := tracker.Records["Player1"]
	if record == nil {
		t.Fatal("Expected record to be created for Player1")
	}
	
	if record.Wins != 0 {
		t.Errorf("Expected 0 wins, got %d", record.Wins)
	}
	
	if record.Losses != 1 {
		t.Errorf("Expected 1 loss, got %d", record.Losses)
	}
	
	if record.WinRate != 0.0 {
		t.Errorf("Expected win rate 0.0, got %f", record.WinRate)
	}
	
	// Test adding a win after a loss
	tracker.AddWin("Player1")
	
	if record.Wins != 1 {
		t.Errorf("Expected 1 win, got %d", record.Wins)
	}
	
	if record.Losses != 1 {
		t.Errorf("Expected 1 loss, got %d", record.Losses)
	}
	
	if record.WinRate != 0.5 {
		t.Errorf("Expected win rate 0.5, got %f", record.WinRate)
	}
}

func TestSortWinners(t *testing.T) {
	tracker := NewWinTracker()
	
	// Add records for multiple players
	tracker.AddWin("Player1")
	tracker.AddWin("Player1")
	tracker.AddLoss("Player1")
	
	tracker.AddWin("Player2")
	tracker.AddWin("Player2")
	tracker.AddWin("Player2")
	
	tracker.AddLoss("Player3")
	tracker.AddLoss("Player3")
	
	tracker.AddWin("Player4")
	tracker.AddLoss("Player4")
	tracker.AddLoss("Player4")
	
	// Sort winners
	sortedRecords := tracker.SortWinners()
	
	if len(sortedRecords) != 4 {
		t.Errorf("Expected 4 records, got %d", len(sortedRecords))
	}
	
	// Player2 should be first (3-0, 100% win rate)
	if sortedRecords[0].PlayerName != "Player2" {
		t.Errorf("Expected Player2 to be first, got %s", sortedRecords[0].PlayerName)
	}
	
	if sortedRecords[0].WinRate != 1.0 {
		t.Errorf("Expected Player2 win rate 1.0, got %f", sortedRecords[0].WinRate)
	}
	
	// Player1 should be second (2-1, 66.7% win rate)
	if sortedRecords[1].PlayerName != "Player1" {
		t.Errorf("Expected Player1 to be second, got %s", sortedRecords[1].PlayerName)
	}
	
	// Player4 should be third (1-2, 33.3% win rate)
	if sortedRecords[2].PlayerName != "Player4" {
		t.Errorf("Expected Player4 to be third, got %s", sortedRecords[2].PlayerName)
	}
	
	// Player3 should be last (0-2, 0% win rate)
	if sortedRecords[3].PlayerName != "Player3" {
		t.Errorf("Expected Player3 to be last, got %s", sortedRecords[3].PlayerName)
	}
	
	if sortedRecords[3].WinRate != 0.0 {
		t.Errorf("Expected Player3 win rate 0.0, got %f", sortedRecords[3].WinRate)
	}
}

func TestWinRateCalculation(t *testing.T) {
	tracker := NewWinTracker()
	
	// Test various win/loss combinations
	tests := []struct {
		playerName   string
		wins         int
		losses       int
		expectedRate float64
	}{
		{"Perfect", 5, 0, 1.0},
		{"Good", 3, 1, 0.75},
		{"Average", 1, 1, 0.5},
		{"Poor", 1, 3, 0.25},
		{"Terrible", 0, 5, 0.0},
	}
	
	for _, test := range tests {
		// Add wins
		for i := 0; i < test.wins; i++ {
			tracker.AddWin(test.playerName)
		}
		
		// Add losses
		for i := 0; i < test.losses; i++ {
			tracker.AddLoss(test.playerName)
		}
		
		record := tracker.Records[test.playerName]
		if record.WinRate != test.expectedRate {
			t.Errorf("Player %s: expected win rate %f, got %f", 
				test.playerName, test.expectedRate, record.WinRate)
		}
	}
}

func TestMultiplePlayersTracking(t *testing.T) {
	tracker := NewWinTracker()
	
	// Simulate a tournament
	players := []string{"Alice", "Bob", "Charlie", "Diana"}
	
	// Add some games
	tracker.AddWin("Alice")
	tracker.AddLoss("Bob")
	
	tracker.AddWin("Charlie")
	tracker.AddLoss("Diana")
	
	tracker.AddWin("Alice")
	tracker.AddLoss("Charlie")
	
	tracker.AddWin("Bob")
	tracker.AddLoss("Alice")
	
	// Check that all players have records
	for _, player := range players {
		if tracker.Records[player] == nil {
			t.Errorf("Expected record for player %s", player)
		}
	}
	
	// Check specific records
	aliceRecord := tracker.Records["Alice"]
	if aliceRecord.Wins != 2 || aliceRecord.Losses != 1 {
		t.Errorf("Alice: expected 2-1, got %d-%d", aliceRecord.Wins, aliceRecord.Losses)
	}
	
	bobRecord := tracker.Records["Bob"]
	if bobRecord.Wins != 1 || bobRecord.Losses != 1 {
		t.Errorf("Bob: expected 1-1, got %d-%d", bobRecord.Wins, bobRecord.Losses)
	}
}
