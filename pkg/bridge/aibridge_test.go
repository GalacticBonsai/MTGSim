package bridge

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestAutoActivateMainPhaseAbilities_NoAbilities_NoPanic(t *testing.T) {
	p1 := game.NewPlayer("A", 20)
	p2 := game.NewPlayer("B", 20)
	g := game.NewGame(p1, p2)

	// Move to Main1 so timing checks pass if consulted by AI
	g.AdvancePhase() // Upkeep
	g.AdvancePhase() // Draw
	g.AdvancePhase() // Main1

	// Should be a no-op (no abilities attached to permanents), but must not panic
	AutoActivateMainPhaseAbilities(g)
}

func TestAutoActivateForPlayer_SpecifiedPhase_NoPanic(t *testing.T) {
	p1 := game.NewPlayer("A", 20)
	p2 := game.NewPlayer("B", 20)
	g := game.NewGame(p1, p2)

	// Even outside of main, calling with explicit phase should be handled
	AutoActivateForPlayer(g, "A", "Combat Phase")
}
