package game

// ApplyStateBasedActions performs a minimal subset:
// - Creatures with lethal damage are put into their owner's graveyard
// - Players with 0 or less life lose the game (marked lost)
// - Auras whose attached object is no longer on the battlefield are put into graveyard
func (g *Game) ApplyStateBasedActions() {
	// 1) Lethal damage / 0-toughness destruction (CR 704.5f, 704.5g, 704.5h)
	// Indestructible (CR 702.12) skips damage-based destruction but still
	// dies from 0-or-less toughness.
	var toGraveyard []*Permanent
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			if !perm.IsCreature() {
				continue
			}
			zeroToughness := perm.GetToughness() <= 0
			lethalDamage := perm.GetDamageCounters() > 0 && perm.GetDamageCounters() >= perm.GetToughness()
			deathtouchLethal := perm.markedLethal && perm.GetDamageCounters() > 0
			if zeroToughness {
				toGraveyard = append(toGraveyard, perm)
				continue
			}
			if perm.HasKeyword(KWIndestructible) {
				continue
			}
			if lethalDamage || deathtouchLethal {
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

	// 3) Legend rule (CR 704.5k): If a player controls two or more legendary permanents with the same name, that player chooses one of them, and the rest are put into their owners' graveyards.
	g.applyLegendRule()

	// 4) Planeswalker uniqueness rule (CR 704.5n): If a player controls two or more planeswalkers with the same planeswalker type, that player chooses one of them, and the rest are put into their owners' graveyards.
	g.applyPlaneswalkerUniquenessRule()

	// 5) Player 0 or less life loses (CR 704.5a)
	for _, pl := range g.players {
		if pl.GetLifeTotal() <= 0 && !pl.HasLost() {
			pl.Lose("life_loss")
		}
	}

	// 6) CR 704.5u: a player who has been dealt 21 or more combat
	// damage by the same commander over the course of the game loses.
	for _, pl := range g.players {
		if pl.MaxCommanderDamageReceived() >= 21 && !pl.HasLost() {
			pl.Lose("commander_damage")
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

// applyLegendRule implements CR 704.5k: If a player controls two or more legendary permanents with the same name, that player chooses one of them, and the rest are put into their owners' graveyards.
// For simulation purposes, we arbitrarily keep the first one encountered.
func (g *Game) applyLegendRule() {
	for _, pl := range g.players {
		nameCounts := make(map[string][]*Permanent)
		for _, perm := range pl.Battlefield {
			if perm.IsLegendary() {
				nameCounts[perm.GetName()] = append(nameCounts[perm.GetName()], perm)
			}
		}
		for _, perms := range nameCounts {
			if len(perms) > 1 {
				// Keep the first one, destroy the rest
				for i := 1; i < len(perms); i++ {
					g.handleDies(perms[i])
				}
			}
		}
	}
}

// applyPlaneswalkerUniquenessRule implements CR 704.5n: If a player controls two or more planeswalkers with the same planeswalker type, that player chooses one of them, and the rest are put into their owners' graveyards.
// For simulation purposes, we arbitrarily keep the first one encountered.
func (g *Game) applyPlaneswalkerUniquenessRule() {
	for _, pl := range g.players {
		typeCounts := make(map[string][]*Permanent)
		for _, perm := range pl.Battlefield {
			if perm.IsPlaneswalker() {
				// Simplified: use the name as the planeswalker type
				typeCounts[perm.GetName()] = append(typeCounts[perm.GetName()], perm)
			}
		}
		for _, perms := range typeCounts {
			if len(perms) > 1 {
				// Keep the first one, destroy the rest
				for i := 1; i < len(perms); i++ {
					g.handleDies(perms[i])
				}
			}
		}
	}
}
