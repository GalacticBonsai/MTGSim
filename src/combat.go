package main

import "strings"

// CanBlock determines if blocker can block attacker, considering abilities like Flying, Reach, Intimidate, Shadow, Fear, etc.
func CanBlock(attacker, blocker Permanant) bool {
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
func AssignBlockers(attacker *Permanant, blockers []*Permanant) bool {
	if CardHasEvergreenAbility(attacker.source, "Menace") {
		if len(blockers) < 2 {
			LogPlayer("%s has Menace and can't be blocked by fewer than two creatures.", attacker.source.Name)
			return false
		}
	}
	if len(blockers) > 0 {
		attacker.blocked = true
		for _, blocker := range blockers {
			blocker.blocking = attacker
		}
	} else {
		attacker.blocked = false
	}
	return true
}

func (p *Player) DealDamage() {
	LogPlayer("First Strike Damage Step")
	for _, creature := range p.Creatures {
		if creature.attacking != nil && (CardHasEvergreenAbility(creature.source, "First Strike") || CardHasEvergreenAbility(creature.source, "Double Strike")) {
			p.resolveCombatDamage(&creature)
		}
	}
	for _, creature := range p.Opponents[0].Creatures {
		if creature.blocking != nil && (CardHasEvergreenAbility(creature.source, "First Strike") || CardHasEvergreenAbility(creature.source, "Double Strike")) {
			p.Opponents[0].resolveCombatDamage(&creature)
		}
	}
	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()
	LogPlayer("Regular Damage Step")
	for _, creature := range p.Creatures {
		if creature.attacking != nil && (!CardHasEvergreenAbility(creature.source, "First Strike")) {
			p.resolveCombatDamage(&creature)
		}
	}
	for _, creature := range p.Opponents[0].Creatures {
		if creature.blocking != nil && !CardHasEvergreenAbility(creature.source, "First Strike") {
			p.Opponents[0].resolveCombatDamage(&creature)
		}
	}
	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()
}

func (p *Player) resolveCombatDamage(creature *Permanant) {
	if creature.blocking != nil {
		if CardHasEvergreenAbility(creature.source, "Deathtouch") {
			LogPlayer("%s deals damage with Deathtouch to %s.", creature.source.Name, creature.blocking.source.Name)
			creature.blocking.damage_counters = creature.blocking.toughness
		} else {
			creature.damages(creature.blocking)
		}
		creature.blocking.damages(creature)
		creature.checkLife()
		creature.blocking.checkLife()
	} else if creature.attacking != nil {
		damage := creature.power
		if CardHasEvergreenAbility(creature.source, "Trample") && creature.blocking != nil {
			excessDamage := damage - creature.blocking.toughness
			if excessDamage > 0 {
				creature.attacking.LifeTotal -= excessDamage
				LogPlayer("%s deals %d excess damage to %s with Trample.", creature.source.Name, excessDamage, creature.attacking.Name)
			}
		} else {
			creature.attacking.LifeTotal -= damage
		}
		LogPlayer("%s deals %d damage to %s", creature.source.Name, damage, creature.attacking.Name)
	}
}

func (p *Player) cleanupDeadCreatures() {
	for i := 0; i < len(p.Creatures); i++ {
		// Indestructible: can't be destroyed
		if CardHasEvergreenAbility(p.Creatures[i].source, "Indestructible") {
			continue
		}
		if p.Creatures[i].toughness <= p.Creatures[i].damage_counters {
			LogPlayer("%s dies due to 0 toughness.", p.Creatures[i].source.Name)
			destroyPermanant(&p.Creatures[i])
			i--
		}
	}
}
