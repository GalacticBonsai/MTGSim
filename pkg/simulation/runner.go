package simulation

import (
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// RunOneTurnAuto advances the game through one full turn (all phases) for the current active player.
// During each main phase, it invokes the ability AI via the bridge (no-op if no abilities exist).
func RunOneTurnAuto(g *game.Game) {
	for steps := 0; steps < 7; steps++ { // Untap -> End -> next Untap
		if g.IsMainPhase() {
			bridge.AutoActivateMainPhaseAbilities(g)
		}
		g.AdvancePhase()
	}
}
