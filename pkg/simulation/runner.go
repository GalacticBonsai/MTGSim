package simulation

import (
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// RunOneTurnAuto advances the game through one full turn for the current
// active player, including the explicit cleanup step.
// During each main phase, it invokes the ability AI via the bridge.
func RunOneTurnAuto(g *game.Game) {
	for steps := 0; steps < 8; steps++ { // Untap -> Cleanup -> next Untap
		if g.IsMainPhase() {
			bridge.AutoActivateMainPhaseAbilities(g)
		}
		g.AdvancePhase()
	}
}
