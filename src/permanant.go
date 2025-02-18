package main

import (
	"fmt"

	"github.com/google/uuid"
)

type PermanantType int

const (
	Creature PermanantType = iota
	Artifact
	Enchantment
	Land
	Planeswalker
)

func (pt PermanantType) String() string {
	switch pt {
	case Creature:
		return "Creature"
	case Artifact:
		return "Artifact"
	case Enchantment:
		return "Enchantment"
	case Land:
		return "Land"
	case Planeswalker:
		return "Planeswalker"
	default:
		return "Unknown"
	}
}

type Permanant struct {
	source            Card
	owner             *Player
	id                uuid.UUID
	tokenType         PermanantType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
	attacking         *Player
	blocking          *Permanant
	blocked           bool
	power             int
	toughness         int
	damage_counters   int
}

func (p Permanant) Display() {
	fmt.Printf("Name: %s, Type: %s\n", p.source.Name, p.tokenType.String())
}

func (p *Permanant) tap() {
	if !p.summoningSickness && !p.tapped {
		p.tapped = true
	}
}

func (p *Permanant) untap() {
	p.tapped = false
}

func DisplayPermanants(permanants []Permanant) {
	if len(permanants) == 0 {
		fmt.Printf("[]\n")
	}
	for _, permanant := range permanants {
		DisplayCard(permanant.source)
	}
}

func (p *Permanant) damages(target *Permanant) {
	fmt.Printf("%s deals %d damage to %s\n", p.source.Name, p.power, target.source.Name)
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
	fmt.Printf("%s sent to player %v's graveyard\n", card.source.Name, p.owner.Name)
	p.owner.Graveyard = append(p.owner.Graveyard, card.source)
}
