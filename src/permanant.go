package main

import (
	"github.com/google/uuid"
)

type Permanant struct {
	source            Card
	owner             *Player
	id                uuid.UUID
	tokenType         PermanantType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
	manaTypes         []ManaType
	attacking         *Player
	blocking          *Permanant
	blockedBy         []*Permanant
	power             int
	toughness         int
	damage_counters   int
	goaded            bool
}

func (p *Permanant) checkManaProducer() {
	p.manaProducer, p.manaTypes = CheckManaProducer(p.source.OracleText)
}

func (p Permanant) Display() {
	LogCard("Name: %s, Type: %s", p.source.Name, p.tokenType)
	if p.manaProducer {
		LogCard("This permanent is a mana producer and produces:")
		for _, manaType := range p.manaTypes {
			LogCard("%s mana", manaType)
		}
	}
}

func (p *Permanant) tap() {
	if !p.tapped {
		p.tapped = true
	}
}

func (p *Permanant) untap() {
	p.tapped = false
}

func DisplayPermanants(permanants []*Permanant) {
	for _, permanant := range permanants {
		DisplayCard(permanant.source)
	}
}

func (p *Permanant) damages(target *Permanant) int{
	LogCard("%s deals %d damage to %s", p.source.Name, p.power, target.source.Name)
	// Handle Lifelink
	if CardHasEvergreenAbility(p.source, "Lifelink") {
		p.owner.LifeTotal += p.power
		LogPlayer("%s deals damage with Lifelink, gaining %d life.", p.source.Name, p.power)
	}
	target.damage_counters += p.power
	if target.damage_counters > target.toughness {
		return target.damage_counters - target.toughness // Overkill
	}
	return 0
}

func (p *Permanant) checkLife() {
	if p.toughness <= p.damage_counters {
		destroyPermanant(p)
	}
}

func (p *Permanant) Fight(other *Permanant) {
	p.damages(other)
	other.damages(p)
}

func destroyPermanant(p *Permanant) {
	if CardHasEvergreenAbility(p.source, "Indestructible") {
		LogCard("%s is indestructible and cannot be destroyed.", p.source.Name)
		return
	}

	// remove permanent from owner's board
	removePermanant := func(list []*Permanant, target *Permanant) []*Permanant {
		for i, c := range list {
			if c == target {
				return append(list[:i], list[i+1:]...)
			}
		}
		return list
	}

	switch p.tokenType {
	case Creature:
		p.owner.Creatures = removePermanant(p.owner.Creatures, p)
	case Land:
		p.owner.Lands = removePermanant(p.owner.Lands, p)
	case Artifact:
		p.owner.Artifacts = removePermanant(p.owner.Artifacts, p)
	case Enchantment:
		p.owner.Enchantments = removePermanant(p.owner.Enchantments, p)
	case Planeswalker:
		p.owner.Planeswalkers = removePermanant(p.owner.Planeswalkers, p)
	}
	
	// add permanent to owner's graveyard
	LogCard("%s sent to player %s's graveyard", p.source.Name, p.owner.Name)
	p.owner.Graveyard = append(p.owner.Graveyard, p.source)
}
