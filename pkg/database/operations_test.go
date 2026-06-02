package database

import (
	"os"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("MTGSIM_TEST_DB")
	if dsn == "" {
		t.Skip("MTGSIM_TEST_DB not set — skipping integration test")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRecordEDHPodAndReadBack(t *testing.T) {
	db := testDB(t)

	pod := EDHPodRecord{
		TotalTurns:        12,
		Winner:            "Test Deck A",
		WinnerCondition:   "combat",
		MaxStormCount:     3,
		TotalManaSpent:    42,
		TotalManaProduced: 50,
		TotalCardsPlayed:  12,
		TotalCombatDamage: 20,
		TotalEliminations: 3,
	}
	players := []EDHPlayerRecord{
		{
			DeckName:       "Test Deck A",
			CommanderName:  "Test Commander A",
			FinalLife:      40,
			CommanderCasts: 2,
			CardsPlayed:    5,
			LandsPlayed:    3,
			SpellsCast:     2,
			CreaturesCast:  1,
			ManaSpent:      15,
			ManaProduced:   20,
			CombatDamage:   20,
			Eliminations:   3,
			MaxStormCount:  3,
		},
		{
			DeckName:       "Test Deck B",
			CommanderName:  "Test Commander B",
			FinalLife:      0,
			Eliminated:     true,
			KillSource:     "combat",
			CardsPlayed:    4,
			LandsPlayed:    2,
			ManaSpent:      12,
			ManaProduced:   15,
			CombatDamage:   5,
		},
		{
			DeckName:       "Test Deck C",
			CommanderName:  "Test Commander C",
			FinalLife:      0,
			Eliminated:     true,
			KillSource:     "life_loss",
			CardsPlayed:    3,
			LandsPlayed:    1,
			ManaSpent:      8,
			ManaProduced:   10,
		},
		{
			DeckName:       "Test Deck D",
			CommanderName:  "Test Commander D",
			FinalLife:      0,
			Eliminated:     true,
			KillSource:     "commander_damage",
			CardsPlayed:    2,
			ManaSpent:      7,
			ManaProduced:   5,
		},
	}
	cardStats := map[string]map[string]struct{ Casts, Wins int }{
		"Test Deck A": {
			"Sol Ring":      {Casts: 3, Wins: 3},
			"Arcane Signet": {Casts: 2, Wins: 2},
			"Lightning Bolt": {Casts: 1, Wins: 0},
		},
		"Test Deck B": {
			"Sol Ring": {Casts: 1, Wins: 0},
		},
	}

	err := db.RecordEDHPod(pod, players, cardStats)
	if err != nil {
		t.Fatalf("RecordEDHPod: %v", err)
	}

	// Read back via EDH deck stats
	stats, err := db.GetEDHDeckStats()
	if err != nil {
		t.Fatalf("GetEDHDeckStats: %v", err)
	}

	found := false
	for _, d := range stats {
		if d.DeckName == "Test Deck A" {
			found = true
			if d.Games < 1 {
				t.Errorf("Test Deck A should have at least 1 game, got %d", d.Games)
			}
			if d.Wins < 1 {
				t.Errorf("Test Deck A should have at least 1 win, got %d", d.Wins)
			}
			if d.CommanderDamageKOs < 1 {
				t.Errorf("expected at least 1 cmdr KO, got %d", d.CommanderDamageKOs)
			}
			if d.LifeLossKOs < 1 {
				t.Errorf("expected at least 1 life-loss KO, got %d", d.LifeLossKOs)
			}
			if len(d.CardStats) == 0 {
				t.Error("expected card stats for Test Deck A")
			} else {
				if s, ok := d.CardStats["Sol Ring"]; !ok || s.Casts == 0 {
					t.Error("expected Sol Ring card stats")
				}
			}
		}
	}
	if !found {
		t.Error("Test Deck A not found in results")
	}

	// Verify summary
	summary, err := db.GetEDHSummary()
	if err != nil {
		t.Fatalf("GetEDHSummary: %v", err)
	}
	if summary.TotalGames < 1 {
		t.Errorf("expected at least 1 total game, got %d", summary.TotalGames)
	}

	// Verify global card stats
	globalStats, err := db.GetGlobalCardStats()
	if err != nil {
		t.Fatalf("GetGlobalCardStats: %v", err)
	}
	solRingFound := false
	for _, gs := range globalStats {
		if gs.CardName == "Sol Ring" {
			solRingFound = true
			if gs.Casts < 4 {
				t.Errorf("Sol Ring should have at least 4 casts across decks, got %d", gs.Casts)
			}
		}
	}
	if !solRingFound {
		t.Error("Sol Ring not found in global card stats")
	}
}

func TestRecordEDHPodMultiplePods(t *testing.T) {
	db := testDB(t)

	for _, deck := range []string{"Deck X", "Deck Y"} {
		pod := EDHPodRecord{
			TotalTurns:        10,
			Winner:            deck,
			TotalManaSpent:    30,
			TotalManaProduced: 40,
			TotalCardsPlayed:  8,
			TotalEliminations: 1,
		}
		players := []EDHPlayerRecord{
			{DeckName: deck, FinalLife: 40, CardsPlayed: 4, LandsPlayed: 2, ManaSpent: 10, ManaProduced: 15, CommanderName: "Cmd " + deck},
			{DeckName: "Other Deck", FinalLife: 0, Eliminated: true, KillSource: "combat", CardsPlayed: 3, ManaSpent: 8, ManaProduced: 10, CommanderName: "Other Cmd"},
		}
		cs := map[string]map[string]struct{ Casts, Wins int }{
			deck: {"Card 1": {Casts: 1, Wins: 1}},
		}
		if err := db.RecordEDHPod(pod, players, cs); err != nil {
			t.Fatalf("RecordEDHPod for %s: %v", deck, err)
		}
	}

	stats, err := db.GetEDHDeckStats()
	if err != nil {
		t.Fatalf("GetEDHDeckStats: %v", err)
	}

	found := 0
	for _, d := range stats {
		if d.DeckName == "Deck X" || d.DeckName == "Deck Y" {
			found++
			if d.Wins < 1 {
				t.Errorf("%s should have 1 win, got %d", d.DeckName, d.Wins)
			}
			if d.Games < 1 {
				t.Errorf("%s should have 1 game, got %d", d.DeckName, d.Games)
			}
		}
	}
	if found < 2 {
		t.Errorf("expected 2 test decks in results, got %d", found)
	}
}
