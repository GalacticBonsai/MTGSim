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
	blocked           bool
	power             int
	toughness         int
	damage_counters   int
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

func DisplayPermanants(permanants []Permanant) {
	for _, permanant := range permanants {
		DisplayCard(permanant.source)
	}
}

func (p *Permanant) damages(target *Permanant) {
	LogCard("%s deals %d damage to %s", p.source.Name, p.power, target.source.Name)
	target.damage_counters += p.power
}

func (p *Permanant) checkLife() {
	if p.toughness <= p.damage_counters {
		destroyPermanant(p)
	}
}

func destroyPermanant(p *Permanant) {
	// remove permanant from owner's board
	var card Permanant
	switch p.tokenType {
	case Creature:
		for i, c := range p.owner.Creatures {
			if c.id == p.id {
				card, p.owner.Creatures = sliceGet(p.owner.Creatures, i)
				break
			}
		}
	case Land:
		for i, c := range p.owner.Lands {
			if c.id == p.id {
				card, p.owner.Lands = sliceGet(p.owner.Lands, i)
				break
			}
		}
	case Artifact:
		for i, c := range p.owner.Artifacts {
			if c.id == p.id {
				card, p.owner.Artifacts = sliceGet(p.owner.Artifacts, i)
				break
			}
		}
	case Enchantment:
		for i, c := range p.owner.Enchantments {
			if c.id == p.id {
				card, p.owner.Enchantments = sliceGet(p.owner.Enchantments, i)
				break
			}
		}
	case Planeswalker:
		for i, c := range p.owner.Planeswalkers {
			if c.id == p.id {
				card, p.owner.Planeswalkers = sliceGet(p.owner.Planeswalkers, i)
				break
			}
		}

	}
	// add permanant to owner's graveyard
	LogCard("%s sent to player %s's graveyard", card.source.Name, p.owner.Name)
	p.owner.Graveyard = append(p.owner.Graveyard, card.source)
}
