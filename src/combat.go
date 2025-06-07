package main

import "strings"

// CanBlock determines if blocker can block attacker, considering abilities like Flying, Reach, Intimidate, Shadow, Fear, etc.
func CanBlock(attacker, blocker *Permanent) bool {
	// Flying: can only be blocked by creatures with flying or reach
	if CardHasEvergreenAbility(attacker.source, "Flying") {
		if CardHasEvergreenAbility(blocker.source, "Flying") || CardHasEvergreenAbility(blocker.source, "Reach") {
			return true
		}
		return false
	}
	// Intimidate: can only be blocked by artifact creatures and/or creatures that share a color
	if CardHasEvergreenAbility(attacker.source, "Intimidate") {
		if strings.Contains(blocker.source.TypeLine, "Artifact") {
			return true
		}
		for _, color := range attacker.source.Colors {
			for _, bcolor := range blocker.source.Colors {
				if color == bcolor {
					return true
				}
			}
		}
		return false
	}
	// Menace: must be blocked by two or more creatures (handled in block assignment logic)
	// Shadow: can only be blocked by creatures with shadow
	if CardHasEvergreenAbility(attacker.source, "Shadow") {
		return CardHasEvergreenAbility(blocker.source, "Shadow")
	}
	// Fear: can only be blocked by artifact creatures and/or black creatures
	if CardHasEvergreenAbility(attacker.source, "Fear") {
		if strings.Contains(blocker.source.TypeLine, "Artifact") {
			return true
		}
		for _, bcolor := range blocker.source.Colors {
			if bcolor == "B" {
				return true
			}
		}
		return false
	}
	// Protection: can't be blocked by creatures with the protected quality
	if CardHasEvergreenAbility(attacker.source, "Protection") {
		for _, kw := range attacker.source.Keywords {
			if strings.HasPrefix(kw, "Protection from ") {
				prot := strings.TrimPrefix(kw, "Protection from ")
				if prot == "Artifacts" && strings.Contains(blocker.source.TypeLine, "Artifact") {
					return false
				}
				if prot == "Black" || prot == "White" || prot == "Blue" || prot == "Red" || prot == "Green" {
					for _, bcolor := range blocker.source.Colors {
						if strings.EqualFold(bcolor, string(prot[0])) {
							return false
						}
					}
				}
			}
		}
	}
	return true
}

// AssignBlockers handles Menace: creatures with Menace must be blocked by two or more creatures if blocked at all.
func AssignBlockers(attacker *Permanent, blockers []*Permanent) bool {
	if CardHasEvergreenAbility(attacker.source, "Menace") {
		if len(blockers) < 2 {
			LogPlayer("%s has Menace and can't be blocked by fewer than two creatures.", attacker.source.Name)
			return false
		}
	}
	for _, blocker := range blockers {
		blocker.blocking = attacker
	}
	attacker.blockedBy = append(attacker.blockedBy, blockers...)
	return true
}

// DealDamage handles the combat damage steps for the player and their opponent.
func (p *Player) DealDamage() {
	LogPlayer("First Strike Damage Step")
	for _, creature := range append(p.Creatures, p.Opponents[0].Creatures...) {
		if (creature.attacking != nil || creature.blocking != nil) && (CardHasEvergreenAbility(creature.source, "First Strike") || CardHasEvergreenAbility(creature.source, "Double Strike")) {
			creature.resolveCombatDamage()
		}
	}

	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()

	LogPlayer("Regular Damage Step")
	for _, creature := range append(p.Creatures, p.Opponents[0].Creatures...) {
		if (creature.attacking != nil || creature.blocking != nil) && (!CardHasEvergreenAbility(creature.source, "First Strike")) {
			creature.resolveCombatDamage()
		}
	}

	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()
}

// resolveCombatDamage resolves combat damage for a permanent.
func (p *Permanent) resolveCombatDamage() {
	if p.attacking != nil {
		var original_power = p.power
		for _, blocker := range p.blockedBy {
			p.power = p.damages(blocker)
		}
		if p.power > 0 && (p.blockedBy == nil || CardHasEvergreenAbility(p.source, "Trample")) {
			p.attacking.LifeTotal -= p.power
			LogPlayer("%s deals %d damage to %s", p.source.Name, p.power, p.attacking.Name)
		}
		p.power = original_power
	}
	if p.blocking != nil {
		p.damages(p.blocking)
	}
}

// cleanupDeadCreatures removes dead creatures from the battlefield after combat damage.
func (p *Player) cleanupDeadCreatures() {
	for _, creature := range p.Creatures {
		// Indestructible: can't be destroyed
		if CardHasEvergreenAbility(creature.source, "Indestructible") {
			continue
		}

		damageSources := creature.blockedBy
		if creature.blocking != nil {
			damageSources = append(damageSources, creature.blocking)
		}

		for _, c := range damageSources {
			if CardHasEvergreenAbility(c.source, "Deathtouch") && c.power > 0 {
				LogPlayer("%s deals damage with Deathtouch to %s.", c, creature)
				destroyPermanent(creature)
			}
		}
		if creature.toughness <= creature.damage_counters {
			LogPlayer("%s dies due to %d damage and %d toughness.", creature.source.Name, creature.damage_counters, creature.toughness)
			destroyPermanent(creature)
		}
	}
}
