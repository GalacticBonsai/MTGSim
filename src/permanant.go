package main

type PermanantType int

const (
	Creature PermanantType = iota
	Artifact
	Enchantment
	Land
	Planeswalker
)

type Permanant struct {
	source            Card
	owner             *Player
	tokenType         PermanantType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
}

func (p *Permanant) tap() {
	if !p.summoningSickness && !p.tapped {
		p.tapped = true
	}
}
