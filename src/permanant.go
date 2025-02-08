package main

import "fmt"

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
	tokenType         PermanantType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
	attacking         *Player
	blocking          *Permanant
	blocked           bool
	power             int
	toughness         int
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
	for _, permanant := range permanants {
		DisplayCard(permanant.source)
	}
}

func (p *Permanant) damages(target *Permanant) {
	fmt.Printf("%s deals %d damage to %s\n", p.source.Name, p.power, target.source.Name)
	target.toughness -= p.power

	fmt.Printf("%s deals %d damage to %s\n", target.source.Name, p.power, p.source.Name)
	p.toughness -= target.power
	if target.toughness <= 0 {
		destroyPermanant(target)
	}
	if p.toughness <= 0 {
		destroyPermanant(p)
	}
}

func destroyPermanant(p *Permanant) {
	// remove permanant from owner's board
	switch p.tokenType {
	case Creature:
		for i, c := range p.owner.Creatures {
			if &c == p {
				_, p.owner.Creatures = sliceGet(p.owner.Creatures, i)
				break
			}
		}
	case Land:
		for i, c := range p.owner.Lands {
			if &c == p {
				_, p.owner.Lands = sliceGet(p.owner.Lands, i)
				break
			}
		}
	case Artifact:
		for i, c := range p.owner.Artifacts {
			if &c == p {
				_, p.owner.Artifacts = sliceGet(p.owner.Artifacts, i)
				break
			}
		}
	case Enchantment:
		for i, c := range p.owner.Enchantments {
			if &c == p {
				_, p.owner.Enchantments = sliceGet(p.owner.Enchantments, i)
				break
			}
		}
	case Planeswalker:
		for i, c := range p.owner.Planeswalkers {
			if &c == p {
				_, p.owner.Planeswalkers = sliceGet(p.owner.Planeswalkers, i)
				break
			}
		}

	}
	// add permanant to owner's graveyard
	p.owner.Graveyard = append(p.owner.Graveyard, p.source)
}
