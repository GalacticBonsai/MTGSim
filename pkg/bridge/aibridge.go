package bridge

import (
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// AutoActivateMainPhaseAbilities runs the ability AI for the active player during the main phase.
// It uses the ability.ExecutionEngine over the AbilityGameState bridge. If there are no
// activatable abilities, this is a no-op.
func AutoActivateMainPhaseAbilities(g *game.Game) {
	gs := NewAbilityGameState(g)
	ai := abil.NewAIDecisionMaker(abil.NewExecutionEngine(gs))
	active := gs.GetActivePlayer()
	if active == nil {
		return
	}
	ai.ActivateAbilitiesForPlayer(active, "Main Phase")
}

// AutoActivateForPlayer runs the AI for a specific player name and phase label.
// Phase labels are advisory for timing checks in the ability engine.
func AutoActivateForPlayer(g *game.Game, playerName, phase string) {
	gs := NewAbilityGameState(g)
	ai := abil.NewAIDecisionMaker(abil.NewExecutionEngine(gs))
	p := gs.GetPlayer(playerName)
	if p == nil {
		return
	}
	ai.ActivateAbilitiesForPlayer(p, phase)
}

