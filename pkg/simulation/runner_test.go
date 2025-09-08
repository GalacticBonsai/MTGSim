package simulation

import (
	"testing"
	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestRunOneTurnAuto_AdvancesAllPhases_NoPanic(t *testing.T) {
	p1 := game.NewPlayer("A", 20)
	p2 := game.NewPlayer("B", 20)
	g := game.NewGame(p1, p2)

	// Initially A is active; after one full turn, active player should rotate to B and phase reset to Untap
	RunOneTurnAuto(g)

	if g.GetActivePlayerRaw() != p2 {
		t.Errorf("expected active player to rotate to B after one turn")
	}
	if g.GetCurrentPhase() != game.PhaseUntap {
		t.Errorf("expected phase to reset to Untap, got %v", g.GetCurrentPhase())
	}
}

