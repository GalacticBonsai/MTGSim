package main

import (
	"fmt"
	"strings"
)

type Player struct {
	Name          string
	LifeTotal     int
	Deck          Deck
	Hand          []Card
	Graveyard     []Card
	Exile         []Card
	Creatures     []Permanant
	Enchantments  []Permanant
	Artifacts     []Permanant
	Planeswalkers []Permanant
	Lands         []Permanant
	// mana          mana
	Opponents []*Player
}

func NewPlayer(decklist string) *Player {
	deck, err := importDeckfile(decklist)
	if err != nil {
		// handle the error appropriately, e.g., log it or return it
		panic(err)
	}
	return &Player{
		Name:      decklist,
		LifeTotal: 20,
		Deck:      deck,
	}
}

func (p *Player) PlayTurn() {
	// p.Display()
	t := newTurn()
	for _, phase := range t.phases {
		for _, s := range phase.steps {
			p.PlayStep(s, t)
		}
	}
}

func (p *Player) PlayStep(s step, t *turn) {
	switch s.name {
	case "Untap Step":
		for i := range p.Lands {
			p.Lands[i].untap()
		}
		for i := range p.Creatures {
			p.Creatures[i].untap()
			p.Creatures[i].summoningSickness = false
		}
		for i := range p.Artifacts {
			p.Artifacts[i].untap()
		}
	case "Upkeep Step":
	case "Draw Step":
		p.Hand = append(p.Hand, p.Deck.DrawCard())
	case "Play Land":
		p.PlayLand(t)
	case "Cast Spells":
		p.PlaySpell()
	case "Beginning of Combat Step":
	case "Declare Attackers Step":
		p.DeclareAttackers()
	case "Declare Blockers Step":
		p.Opponents[0].DeclareBlockers()
	case "Combat Damage Step":
		p.DealDamage()
	case "End of Combat Step":
		p.CleanupCombat()
		p.Opponents[0].CleanupCombat()
	case "End Step":
		p.EndStep()
		p.Opponents[0].EndStep()
	case "Cleanup Step":
		// discard down to 7
		var c Card
		for len(p.Hand) > 7 {
			c, p.Hand = sliceGet(p.Hand, 0)
			LogPlayer("Discarding: %s", c.Name)
			p.Graveyard = append(p.Graveyard, c)
		}
	}
}

func (p *Player) CleanupCombat() {
	for i := range p.Creatures {
		p.Creatures[i].attacking = nil
		p.Creatures[i].blocking = nil
		p.Creatures[i].blocked = false
	}
}

func (p *Player) DealDamage() {
	for _, creature := range p.Opponents[0].Creatures {
		if creature.blocking != nil {
			creature.damages(creature.blocking)
			creature.blocking.damages(&creature)
			creature.checkLife()
			creature.blocking.checkLife()
		}
	}
	for _, creature := range p.Creatures {

		if creature.blocked || creature.attacking == nil {
			continue
		}

		creature.attacking.LifeTotal -= creature.power
		LogPlayer("%s deals %d damage to %s", creature.source.Name, creature.power, creature.attacking.Name)
	}
}

func (p *Player) DeclareBlockers() {
	for i, creature := range p.Creatures {
		if creature.tapped {
			continue
		}
		for j, attacker := range p.Opponents[0].Creatures {
			if attacker.attacking == p && !attacker.blocked {
				p.Creatures[i].blocking = &p.Opponents[0].Creatures[j]
				p.Opponents[0].Creatures[j].blocked = true
				LogPlayer("%s blocked by %s", attacker.source.Name, creature.source.Name)
				break // exit out to not block all attackers
			}
		}
	}
}

func (p *Player) DeclareAttackers() {
	LogPlayer("Declare attacker:")
	attacking := false
	for i, creature := range p.Creatures {
		if creature.tapped || creature.summoningSickness {
			continue
		}

		creature.Display()
		p.Creatures[i].attacking = p.Opponents[0]
		attacking = true
	}
	if !attacking {
		LogPlayer("None")
	}
}

func (p *Player) PlayLand(t *turn) {
	for i := 0; i < len(p.Hand); i++ {
		c := p.Hand[i]
		if t.landPerTurn <= 0 {
			return
		}
		if strings.Contains(c.TypeLine, "Land") {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			LogCard("Playing land: %s", c.Name)

			// adds land to board
			land := Permanant{
				source:            c,
				owner:             p,
				tokenType:         Land,
				tapped:            false,
				summoningSickness: false,
			}
			land.checkManaProducer()
			p.Lands = append(p.Lands, land)

			// pops card from hand
			t.landPerTurn--
			i-- // Adjust index after removing an element
		}
	}
}

// fix for colors
func (p *Player) ManaAvailable() mana {
	totalMana := newMana()
	for _, c := range p.Lands {
		if !c.tapped {
			for _, manaType := range c.manaTypes {
				totalMana.pool[manaType]++
			}
		}
	}
	for _, c := range p.Creatures {
		if !c.tapped && c.manaProducer && !c.summoningSickness {
			for _, manaType := range c.manaTypes {
				totalMana.pool[manaType]++
			}
		}
	}
	for _, c := range p.Artifacts {
		if !c.tapped && c.manaProducer {
			for _, manaType := range c.manaTypes {
				totalMana.pool[manaType]++
			}
		}
	}
	return totalMana
}

func (p *Player) PlaySpell() {
	for i := 0; i < len(p.Hand); i++ {
		c := p.Hand[i]
		// check if spell can be cast
		if strings.Contains(c.TypeLine, "Land") {
			continue
		}

		// check if mana available
		cost := ParseManaCost(c.ManaCost)
		err := p.tapForMana(cost)
		if err != nil {
			continue
		}

		LogCard("Casting spell: %s", c.Name)
		// pops card from hand
		p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
		c.Cast(nil, p)
		i-- // Adjust index after removing an element
	}
}

func (p *Player) tapForMana(cost mana) error {
	availableMana := p.ManaAvailable()
	if availableMana.total() < cost.total() {
		return fmt.Errorf("not enough mana available")
	}

	// tap lands for mana
	for i := range p.Lands {
		if !p.Lands[i].tapped {
			for _, manaType := range p.Lands[i].manaTypes {
				if cost.pool[manaType] > 0 {
					p.Lands[i].tap()
					cost.pool[manaType]--
				}
			}
		}
		if cost.total() == 0 {
			return nil
		}
	}

	// tap creatures for mana
	for i, c := range p.Creatures {
		if !c.tapped && !c.summoningSickness && c.manaProducer {
			for _, manaType := range c.manaTypes {
				if cost.pool[manaType] > 0 {
					p.Creatures[i].tap()
					cost.pool[manaType]--
				}
			}
		}
		if cost.total() == 0 {
			return nil
		}
	}

	// tap artifacts for mana
	for i, a := range p.Artifacts {
		if !a.tapped {
			for _, manaType := range a.manaTypes {
				if cost.pool[manaType] > 0 {
					p.Artifacts[i].tap()
					cost.pool[manaType]--
				}
			}
		}
		if cost.total() == 0 {
			return nil
		}
	}

	return nil
}

func (p *Player) Display() {
	LogPlayer("Player: %s", p.Name)
	LogPlayer("Life: %d", p.LifeTotal)
	LogPlayer("Hand:")
	DisplayCards(p.Hand)
	LogPlayer("Board:")
	DisplayPermanants(p.Creatures)
	DisplayPermanants(p.Lands)
}

func (p *Player) EndStep() {
	for i := range p.Creatures {
		p.Creatures[i].damage_counters = 0
	}
}
