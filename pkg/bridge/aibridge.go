package bridge

import (
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// AutoActivateMainPhaseAbilities runs the ability AI for the active player during the main phase.
func AutoActivateMainPhaseAbilities(g *game.Game) {
	gs := NewAbilityGameState(g)
	ai := abil.NewAIDecisionMaker(abil.NewExecutionEngine(gs))
	active := gs.GetActivePlayer()
	if active == nil {
		return
	}
	ai.ActivateAbilitiesForPlayer(active, "Main Phase")
}

// AutoActivateMainPhaseAbilitiesWithLog is like AutoActivateMainPhaseAbilities but logs
// ability activations via the provided callback.
func AutoActivateMainPhaseAbilitiesWithLog(g *game.Game, onActivate func(cardName, detail string)) {
	gs := NewAbilityGameState(g)
	gs.OnActivate = onActivate
	ai := abil.NewAIDecisionMaker(abil.NewExecutionEngine(gs))
	active := gs.GetActivePlayer()
	if active == nil {
		return
	}
	ai.ActivateAbilitiesForPlayer(active, "Main Phase")
}

// AutoActivateForPlayer runs the AI for a specific player name and phase label.
func AutoActivateForPlayer(g *game.Game, playerName, phase string) {
	gs := NewAbilityGameState(g)
	ai := abil.NewAIDecisionMaker(abil.NewExecutionEngine(gs))
	p := gs.GetPlayer(playerName)
	if p == nil {
		return
	}
	ai.ActivateAbilitiesForPlayer(p, phase)
}
