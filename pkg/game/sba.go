package game

// ApplyStateBasedActions performs a minimal subset:
// - Creatures with lethal damage are put into their owner's graveyard
// - Players with 0 or less life lose the game (marked lost)
// - Auras whose attached object is no longer on the battlefield are put into graveyard
func (g *Game) ApplyStateBasedActions() {
	// 1) Lethal damage
	var toGraveyard []*Permanent
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			if perm.IsCreature() && perm.GetDamageCounters() >= perm.GetToughness() {
				toGraveyard = append(toGraveyard, perm)
			}
		}
	}
	for _, perm := range toGraveyard {
		// remove from battlefield applying replacement effects if any
		g.handleDies(perm)
	}

	// 2) Aura detach: any aura whose attached target left the battlefield goes to graveyard
	var aurasToGY []*Permanent
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			if perm.IsAura() {
				t := perm.GetAttachedTo()
				if t == nil || !g.onBattlefield(t) {
					aurasToGY = append(aurasToGY, perm)
				}
			}
		}
	}
	for _, aura := range aurasToGY {
		if ctrl := aura.GetController(); ctrl != nil {
			ctrl.DestroyPermanent(aura)
		}
	}

	// 3) Player 0 or less life loses
	for _, pl := range g.players {
		if pl.GetLifeTotal() <= 0 {
			pl.lost = true
		}
	}
}

func (g *Game) onBattlefield(p *Permanent) bool {
	for _, pl := range g.players {
		for _, bp := range pl.Battlefield {
			if bp == p {
				return true
			}
		}
	}
	return false
}
