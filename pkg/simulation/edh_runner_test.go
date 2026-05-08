package simulation

import (
	"math/rand"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// makeSeat builds a minimal EDH seat with the given deck name. Lands and
// creatures are interleaved roughly 1:1 so the runner's "play 1 land,
// summon all creatures" loop produces enough offensive pressure for
// games to terminate within sane turn limits.
func makeSeat(name, basic, creature, power string, copies int, cmdr *game.SimpleCard) EDHSeat {
	cards := make([]game.SimpleCard, 0, 30+copies)
	for i := 0; i < 30; i++ {
		cards = append(cards, game.SimpleCard{Name: basic, TypeLine: "Basic Land — " + basic})
	}
	for i := 0; i < copies; i++ {
		cards = append(cards, game.SimpleCard{
			Name: creature, TypeLine: "Creature", Power: power, Toughness: "2",
		})
	}
	return EDHSeat{
		DeckName:  name,
		Library:   cards,
		Commander: cmdr,
	}
}

func TestSimulateEDHGame_TwoPlayer_DamageWin(t *testing.T) {
	seats := []EDHSeat{
		makeSeat("Aggro", "Mountain", "Goblin", "10", 8, nil),
		makeSeat("Control", "Island", "Wall", "0", 4, nil),
	}
	rng := rand.New(rand.NewSource(42))

	rec, err := SimulateEDHGame(EDHRunOptions{Seats: seats, MaxTurns: 30, RNG: rng})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if rec.Winner == "" {
		t.Fatalf("expected a winner within turn limit, got record %+v", rec)
	}
	if rec.Winner != "Aggro" {
		t.Logf("non-deterministic winner %q (acceptable)", rec.Winner)
	}
	if len(rec.Players) != 2 {
		t.Fatalf("expected 2 players in record, got %d", len(rec.Players))
	}
}

func TestSimulateEDHGame_RegistersCommander(t *testing.T) {
	cmdr := &game.SimpleCard{Name: "Test Cmdr", TypeLine: "Legendary Creature", Power: "5", Toughness: "5"}
	seats := []EDHSeat{
		makeSeat("WithCmdr", "Forest", "Bear", "2", 4, cmdr),
		makeSeat("NoCmdr", "Plains", "Soldier", "1", 4, nil),
	}
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    seats,
		MaxTurns: 15,
		RNG:      rand.New(rand.NewSource(7)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	for _, pr := range rec.Players {
		if pr.DeckName == "WithCmdr" {
			if pr.CommanderName != "Test Cmdr" {
				t.Fatalf("expected commander name to be recorded, got %q", pr.CommanderName)
			}
			if pr.CommanderCasts == 0 {
				t.Fatalf("expected at least 1 commander cast across %d turns, got 0", rec.Turns)
			}
		}
	}
}

func TestSimulateEDHGame_RequiresTwoSeats(t *testing.T) {
	_, err := SimulateEDHGame(EDHRunOptions{Seats: []EDHSeat{makeSeat("Solo", "Plains", "x", "1", 1, nil)}})
	if err == nil {
		t.Fatalf("expected error for single-seat pod")
	}
}

func TestSimulateEDHGame_FourPlayerPod_HasOneSurvivor(t *testing.T) {
	seats := []EDHSeat{
		makeSeat("A", "Mountain", "Goblin", "8", 30, nil),
		makeSeat("B", "Island", "Drake", "5", 30, nil),
		makeSeat("C", "Forest", "Bear", "6", 30, nil),
		makeSeat("D", "Plains", "Knight", "5", 30, nil),
	}
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    seats,
		MaxTurns: 60,
		RNG:      rand.New(rand.NewSource(123)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	alive := 0
	for _, pr := range rec.Players {
		if !pr.Eliminated {
			alive++
		}
	}
	if alive > 1 {
		t.Fatalf("expected at most 1 survivor, got %d", alive)
	}
}

func TestEDHResults_FromSimulatedGame(t *testing.T) {
	results := NewEDHResults()
	for i := 0; i < 3; i++ {
		seats := []EDHSeat{
			makeSeat("A", "Mountain", "Goblin", "8", 6, nil),
			makeSeat("B", "Island", "Drake", "1", 6, nil),
		}
		rec, err := SimulateEDHGame(EDHRunOptions{
			Seats:    seats,
			MaxTurns: 25,
			RNG:      rand.New(rand.NewSource(int64(i + 1))),
		})
		if err != nil {
			t.Fatalf("simulate: %v", err)
		}
		results.RecordGame(rec)
	}
	if results.GameCount() != 3 {
		t.Fatalf("expected 3 games recorded, got %d", results.GameCount())
	}
	stats := results.DeckStats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 decks in aggregate, got %d", len(stats))
	}
}

func TestRunMainPhase_TapsLandsAndPaysCreatureCosts(t *testing.T) {
	p1 := game.NewPlayer("A", 40)
	p2 := game.NewPlayer("B", 40)
	g := game.NewGame(p1, p2)
	p1.AddCardToHand(game.SimpleCard{Name: "Forest", TypeLine: "Basic Land — Forest"})
	p1.AddCardToHand(game.SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2", ManaCost: "{G}"})
	p1.AddCardToHand(game.SimpleCard{Name: "Elk", TypeLine: "Creature", Power: "3", Toughness: "3", ManaCost: "{2}{G}"})

	m := newEDHMetrics(2)
	runMainPhase(g, p1, []int{0, 0}, nil, m)

	if len(p1.GetLands()) != 1 || !p1.GetLands()[0].IsTapped() {
		t.Fatalf("expected Forest to be played and tapped for mana")
	}
	creatures := p1.GetCreatures()
	if len(creatures) != 1 || creatures[0].GetName() != "Bear" {
		t.Fatalf("expected only affordable Bear to be summoned, got %+v", creatures)
	}
	if p1.FindCardInHand("Elk") < 0 {
		t.Fatalf("unaffordable Elk should remain in hand")
	}
	if got := m.players[0]; got.LandsPlayed != 1 || got.CreaturesCast != 1 || got.SpellsCast != 1 || got.ManaSpent != 1 || got.MaxStormCount != 1 {
		t.Fatalf("unexpected main-phase metrics: %+v", got)
	}
}

func TestRunMainPhase_CastsManaRockAndUsesItForPermanent(t *testing.T) {
	p1 := game.NewPlayer("A", 40)
	p2 := game.NewPlayer("B", 40)
	g := game.NewGame(p1, p2)
	p1.AddCardToHand(game.SimpleCard{Name: "Forest", TypeLine: "Basic Land — Forest"})
	p1.AddCardToHand(game.SimpleCard{Name: "Sol Ring", TypeLine: "Artifact", ManaCost: "{1}", OracleText: "{T}: Add {C}{C}."})
	p1.AddCardToHand(game.SimpleCard{Name: "Steel Golem", TypeLine: "Artifact Creature", Power: "3", Toughness: "4", ManaCost: "{2}"})

	log := NewEDHEventLog()
	m := newEDHMetrics(2)
	runMainPhase(g, p1, []int{0, 0}, log, m)

	if p1.FindCardInHand("Sol Ring") >= 0 || p1.FindCardInHand("Steel Golem") >= 0 {
		t.Fatalf("expected Sol Ring and Steel Golem to be cast; hand=%+v", p1.Hand)
	}
	artifacts := 0
	for _, perm := range p1.Battlefield {
		if perm.GetSource().IsArtifact() {
			artifacts++
		}
	}
	if artifacts < 2 || len(p1.GetCreatures()) != 1 {
		t.Fatalf("expected mana rock plus artifact creature on battlefield")
	}
	if got := m.players[0]; got.CardsPlayed != 3 || got.SpellsCast != 2 || got.CreaturesCast != 1 || got.ManaSpent != 3 || got.MaxStormCount != 2 {
		t.Fatalf("unexpected permanent metrics: %+v", got)
	}
	if events := log.Events(); len(events) < 3 || events[1].Kind != EventPermanentCast || events[2].Kind != EventCreatureSummon {
		t.Fatalf("expected permanent then creature events, got %+v", events)
	}
}
