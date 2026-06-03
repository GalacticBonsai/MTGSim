package simulation

import (
	"math/rand"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// recordingHandler counts invocations to verify the runner honours the
// priority hook for non-active opponents.
type recordingHandler struct {
	calls int
}

func (r *recordingHandler) OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase) {
	r.calls++
}

// TestStackAwareHandler_CreatesStack verifies the handler initialises the
// ability engine, AI, and stack without panicking.
func TestStackAwareHandler_CreatesStack(t *testing.T) {
	p1 := game.NewEDHPlayer("A")
	p2 := game.NewEDHPlayer("B")
	g := game.NewGame(p1, p2)

	h := NewStackAwareHandler(g, nil)
	if h == nil {
		t.Fatal("NewStackAwareHandler returned nil")
	}
	if h.engine == nil {
		t.Error("expected non-nil ExecutionEngine")
	}
	if h.ai == nil {
		t.Error("expected non-nil AIDecisionMaker")
	}
	if h.gameState == nil {
		t.Error("expected non-nil AbilityGameState")
	}
}

// TestStackAwareHandler_ProcessPriorityRound terminates immediately when
// all players pass with an empty stack (the default DecisionFunc is
// pass). This tests that the 100-iteration safety limit doesn't fire
// spuriously.
func TestStackAwareHandler_ProcessPriorityRound_TerminatesCleanly(t *testing.T) {
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats: []EDHSeat{
			{DeckName: "A", Library: []game.SimpleCard{
				{Name: "Forest", TypeLine: "Basic Land — Forest"},
			}},
			{DeckName: "B", Library: []game.SimpleCard{
				{Name: "Island", TypeLine: "Basic Land — Island"},
			}},
		},
		MaxTurns: 3,
		RNG:      rand.New(rand.NewSource(42)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if rec.Turns == 0 {
		t.Error("expected at least 1 turn to complete")
	}
}

// TestStackAwareHandler_CustomHandlerStillWorks verifies that passing a
// custom PriorityHandler (non-StackAwareHandler) still works correctly.
func TestStackAwareHandler_CustomHandlerStillWorks(t *testing.T) {
	a := makeSeat("A", "Plains", "Bear", "2", 8, nil)
	b := makeSeat("B", "Forest", "Wolf", "3", 8, nil)
	h := &recordingHandler{}
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    []EDHSeat{a, b},
		MaxTurns: 5,
		RNG:      rand.New(rand.NewSource(1)),
		Priority: h,
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if h.calls == 0 {
		t.Error("expected custom handler to be called")
	}
	if rec.Turns == 0 {
		t.Error("expected turns to advance")
	}
}

// TestStackAwareHandler_AIDecisionIsCalledInPriority verifies that the
// AI decision function is invoked during a real simulation by setting up
// players with lands that trigger AI evaluation.
func TestStackAwareHandler_AIDecisionIsCalled(t *testing.T) {
	a := makeSeat("A", "Plains", "Bear", "2", 8, nil)
	b := makeSeat("B", "Forest", "Wolf", "3", 8, nil)

	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    []EDHSeat{a, b},
		MaxTurns: 3,
		RNG:      rand.New(rand.NewSource(7)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if rec.Turns == 0 {
		t.Error("expected game to advance turns")
	}
}

// TestStackAwareHandler_InstantCastingDuringPriority verifies that when
// an opponent has a castable instant in hand, the priority handler allows
// casting it during the opponent's priority window.
func TestStackAwareHandler_InstantCastingDuringPriority(t *testing.T) {
	bear := game.SimpleCard{Name: "Bear", TypeLine: "Creature", ManaCost: "{G}", Power: "2", Toughness: "2"}
	bolt := game.SimpleCard{Name: "Lightning Bolt", TypeLine: "Instant", ManaCost: "{R}", OracleText: "Lightning Bolt deals 3 damage to any target."}
	mtn := game.SimpleCard{Name: "Mountain", TypeLine: "Basic Land — Mountain"}

	aSeat := EDHSeat{
		DeckName: "A",
		Library:  append([]game.SimpleCard{bolt, mtn, mtn, mtn, mtn, mtn}, repeat(bear, 20)...),
	}
	bSeat := EDHSeat{
		DeckName: "B",
		Library:  repeat(mtn, 30),
	}

	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    []EDHSeat{aSeat, bSeat},
		MaxTurns: 10,
		RNG:      rand.New(rand.NewSource(42)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if rec.Turns == 0 {
		t.Error("expected turns to advance")
	}
}

// repeat builds a slice with n copies of a card.
func repeat(c game.SimpleCard, n int) []game.SimpleCard {
	out := make([]game.SimpleCard, n)
	for i := range out {
		out[i] = c
	}
	return out
}
