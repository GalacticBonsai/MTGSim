package simulation

import "github.com/mtgsim/mtgsim/pkg/game"

// classifyElimination attributes an eliminated player's loss to the most
// specific applicable rule. Order matters: commander damage (CR 704.5u)
// is checked before generic life loss because both are typically true
// when a commander deals 21+ damage.
func classifyElimination(p *game.Player, turnLimitHit bool) KillSource {
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
