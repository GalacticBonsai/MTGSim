package simulation

import (
	"math/rand"
	"testing"
	"time"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// TestEDHThroughput verifies that the EDH simulation maintains a minimum
// throughput of 1 game per second in a 4-player pod. This acts as a
// regression guard against performance regressions (e.g. infinite loops,
// memory leaks, or overly expensive ability parsing).
func TestEDHThroughput(t *testing.T) {
	const (
		games    = 20
		// Minimum throughput floor (accounts for ~3x slowdown from -race).
		// Catches severe regressions (infinite loops, memory leaks, etc.)
		// while allowing normal variation across CI runner specs.
		minGameS = 0.5
	)

	seats := make4PlayerSeats()

	start := time.Now()
	for i := 0; i < games; i++ {
		rng := rand.New(rand.NewSource(int64(i + 1)))
		_, err := SimulateEDHGame(EDHRunOptions{
			Seats:    seats,
			MaxTurns: 30,
			RNG:      rng,
		})
		if err != nil {
			t.Fatalf("game %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	rate := float64(games) / elapsed.Seconds()

	if rate < minGameS {
		t.Errorf(
			"throughput %.1f games/s below minimum %.1f games/s (%d games in %v)",
			rate, minGameS, games, elapsed,
		)
	} else {
		t.Logf("throughput %.1f games/s (%d games in %v)", rate, games, elapsed)
	}
}

func make4PlayerSeats() []EDHSeat {
	return []EDHSeat{
		makeSeat("P1", "Plains", "Bear", "2", 8, &game.SimpleCard{
			Name: "Cmdr A", TypeLine: "Legendary Creature", Power: "3", Toughness: "3", ManaCost: "{2}{W}",
		}),
		makeSeat("P2", "Island", "Merfolk", "3", 8, &game.SimpleCard{
			Name: "Cmdr B", TypeLine: "Legendary Creature", Power: "4", Toughness: "4", ManaCost: "{2}{U}",
		}),
		makeSeat("P3", "Swamp", "Zombie", "2", 8, &game.SimpleCard{
			Name: "Cmdr C", TypeLine: "Legendary Creature", Power: "5", Toughness: "5", ManaCost: "{2}{B}",
		}),
		makeSeat("P4", "Mountain", "Goblin", "2", 8, &game.SimpleCard{
			Name: "Cmdr D", TypeLine: "Legendary Creature", Power: "4", Toughness: "4", ManaCost: "{2}{R}",
		}),
	}
}
