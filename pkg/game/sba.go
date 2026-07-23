package game

// ApplyStateBasedActions performs the comprehensive SBA check per CR 704.
// It loops until no further actions are taken (CR 704.3).
func (g *Game) ApplyStateBasedActions() {
	for {
		changed := g.applyStateBasedActionsOnce()
		if !changed {
			return
		}
	}
}

// applyStateBasedActionsOnce applies one pass of SBAs and returns true if
// any game state changed.
func (g *Game) applyStateBasedActionsOnce() bool {
	changed := false

	// --- CR 704.5a: player with <= 0 life loses ---
	for _, pl := range g.players {
		if pl.GetLifeTotal() <= 0 && !pl.HasLost() {
			pl.Lose("life_loss")
			changed = true
		}
	}

	// --- CR 704.5b: player with no cards in library who tries to draw loses ---
	// (handled in drawStep)

	// --- CR 704.5c: player with 10+ poison counters loses ---
	for _, pl := range g.players {
		if pl.PoisonCounters() >= 10 && !pl.HasLost() {
			pl.Lose("poison")
			changed = true
		}
	}

	// --- CR 704.5d/704.5e/704.5i: +1/+1 and -1/-1 counters cancel ---
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			if perm.removeCancelledCounters() {
				changed = true
			}
		}
	}

	// --- CR 704.5f/704.5g/704.5h: creatures with 0 or less toughness or lethal damage die ---
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
		g.handleDies(perm)
		changed = true
	}

	// --- CR 704.5j: planeswalker with 0 or less loyalty is put into its owner's graveyard ---
	var pwToGraveyard []*Permanent
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			if perm.IsPlaneswalker() && perm.GetCounters("loyalty") <= 0 {
				pwToGraveyard = append(pwToGraveyard, perm)
			}
		}
	}
	for _, perm := range pwToGraveyard {
		g.handleDies(perm)
		changed = true
	}

	// --- CR 704.5k: Legend rule (same legendary permanent name) ---
	changed = g.applyLegendRule() || changed

	// --- CR 704.5n: Planeswalker uniqueness rule (same planeswalker type) ---
	changed = g.applyPlaneswalkerUniquenessRule() || changed

	// --- CR 704.5u: commander damage (21+) ---
	for _, pl := range g.players {
		if pl.MaxCommanderDamageReceived() >= 21 && !pl.HasLost() {
			pl.Lose("commander_damage")
			changed = true
		}
	}

	// --- Aura detach: any aura whose attached target left the battlefield ---
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
			changed = true
		}
	}

	return changed
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
func (g *Game) applyLegendRule() bool {
	changed := false
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
					changed = true
				}
			}
		}
	}
	return changed
}

// applyPlaneswalkerUniquenessRule implements CR 704.5n: If a player controls two or more planeswalkers with the same planeswalker type, that player chooses one of them, and the rest are put into their owners' graveyards.
// The uniqueness check uses the planeswalker supertype (e.g. "Jace"), not the full card name.
func (g *Game) applyPlaneswalkerUniquenessRule() bool {
	changed := false
	for _, pl := range g.players {
		typeCounts := make(map[string][]*Permanent)
		for _, perm := range pl.Battlefield {
			if perm.IsPlaneswalker() {
				pwType := perm.source.PlaneswalkerType()
				if pwType != "" {
					typeCounts[pwType] = append(typeCounts[pwType], perm)
				}
			}
		}
		for _, perms := range typeCounts {
			if len(perms) > 1 {
				// Keep the first one, destroy the rest
				for i := 1; i < len(perms); i++ {
					g.handleDies(perms[i])
					changed = true
				}
			}
		}
	}
	return changed
}
