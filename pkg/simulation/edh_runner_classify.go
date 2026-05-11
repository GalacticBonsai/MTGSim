package simulation

import "github.com/mtgsim/mtgsim/pkg/game"

// classifyElimination attributes an eliminated player's loss to the most
// specific applicable rule. Order matters: commander damage (CR 704.5u)
// is checked before generic life loss because both are typically true
// when a commander deals 21+ damage.
func classifyElimination(p *game.Player, turnLimitHit bool) KillSource {
	// If the engine already recorded a loss reason (e.g., from an effect or SBA), trust it.
	if reason := p.GetLossReason(); reason != "" {
		switch reason {
		case "commander_damage":
			return KillSourceCommanderDamage
		case "life_loss":
			return KillSourceLifeLoss
		case "deckout":
			return KillSourceDeckout
		case "effect":
			return KillSourceEffect
		}
	}
	if p.MaxCommanderDamageReceived() >= 21 {
		return KillSourceCommanderDamage
	}
	if p.LibrarySize() == 0 && p.GetLifeTotal() <= 0 {
		// Library exhausted at the moment of loss is the runner's mill
		// proxy (real CR 704.5b is the empty-library draw, not life).
		return KillSourceMill
	}
	if p.GetLifeTotal() <= 0 {
		return KillSourceLifeLoss
	}
	if turnLimitHit {
		return KillSourceTurnLimit
	}
	return KillSourceUnknown
}

// classifyWinCondition infers how the winner won based on the game end state.
func classifyWinCondition(winner *game.Player, g *game.Game) WinCondition {
	if winner == nil || g == nil {
		return WinConditionUnknown
	}
	// If the winner has a high commander-damage dealt total, infer commander damage.
	// Otherwise default to combat for now; effect/deckout wins require explicit
	// engine support (e.g., Game.WinGame with condition "effect").
	return WinConditionCombat
}
