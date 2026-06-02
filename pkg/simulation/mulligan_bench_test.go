package simulation

import (
	"math/rand"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func BenchmarkEvaluateOpeningHand(b *testing.B) {
	hand := []game.SimpleCard{
		{Name: "Island", TypeLine: "Basic Land — Island"},
		{Name: "Swamp", TypeLine: "Basic Land — Swamp"},
		{Name: "Sol Ring", TypeLine: "Artifact", ManaCost: "{1}"},
		{Name: "Arcane Signet", TypeLine: "Artifact", ManaCost: "{2}"},
		{Name: "Ad Nauseam", TypeLine: "Sorcery", ManaCost: "{3}{B}{B}"},
		{Name: "Force of Will", TypeLine: "Instant", ManaCost: "{3}{U}{U}"},
		{Name: "Mystic Remora", TypeLine: "Enchantment", ManaCost: "{U}"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluateOpeningHand(hand, nil, 0, 0)
	}
}

func BenchmarkIterativeMulligan(b *testing.B) {
	cards := make([]game.SimpleCard, 99)
	for i := 0; i < 99; i++ {
		if i < 30 {
			cards[i] = game.SimpleCard{Name: "Island", TypeLine: "Basic Land — Island"}
		} else {
			cards[i] = game.SimpleCard{Name: "Counterspell", TypeLine: "Instant", ManaCost: "{U}{U}"}
		}
	}
	p := game.NewEDHPlayer("test")
	p.Library = append(p.Library, cards...)

	rng := rand.New(rand.NewSource(42))
	p.DrawOpeningHand()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := game.NewEDHPlayer("test")
		p.Library = append(p.Library, cards...)
		rng.Shuffle(len(p.Library), func(a, b int) { p.Library[a], p.Library[b] = p.Library[b], p.Library[a] })
		p.DrawOpeningHand()
		iterativeMulligan(p, rng, 0, nil)
	}
}

func BenchmarkFullGameSimulation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rng := rand.New(rand.NewSource(42))
		solRing := game.SimpleCard{Name: "Sol Ring", TypeLine: "Artifact", ManaCost: "{1}"}
		seats := []EDHSeat{
			makeSeat("Alice", "Island", "Faerie", "1", 30, &solRing),
			makeSeat("Bob", "Mountain", "Goblin", "2", 30, &solRing),
			makeSeat("Carol", "Swamp", "Zombie", "2", 30, &solRing),
			makeSeat("Dave", "Forest", "Elf", "1", 30, &solRing),
		}
		b.StartTimer()

		_, err := SimulateEDHGame(EDHRunOptions{
			Seats:    seats,
			MaxTurns: 30,
			RNG:      rng,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
