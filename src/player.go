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
	mana          mana
	Opponents     []*Player
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
			fmt.Printf("Discarding: %s\n", c.Name)
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
		fmt.Printf("%s deals %d damge to %s\n", creature.source.Name, creature.power, creature.attacking.Name)
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
				fmt.Printf("%s blocked by %s\n", attacker.source.Name, creature.source.Name)
				break // exit out to not block all attackers
			}
		}
	}
}

func (p *Player) DeclareAttackers() {
	fmt.Println("Declare attacker: ")
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
		fmt.Println("None")
	}
}

func (p *Player) PlayLand(t *turn) {
	for i, c := range p.Hand {
		if t.landPerTurn <= 0 {
			return
		}
		if strings.Contains(c.TypeLine, "Land") {
			_, p.Hand = sliceGet(p.Hand, i)
			fmt.Println("Playing land: ")
			c.Display()

			// adds land to board
			p.Lands = append(p.Lands, Permanant{
				source:            c,
				owner:             p,
				tokenType:         Land,
				manaProducer:      true,
				tapped:            false,
				summoningSickness: false,
			})

			// pops card from hand
			t.landPerTurn--
		}
	}
}

// fix for colors
func (p *Player) ManaAvailable() int {
	totalMana := 0
	for _, c := range p.Lands {
		if !c.tapped {
			totalMana++
		}
	}
	for _, c := range p.Creatures {
		if !c.tapped && c.manaProducer && !c.summoningSickness {
			totalMana++
		}
	}
	for _, c := range p.Artifacts {
		if !c.tapped && c.manaProducer {
			totalMana++
		}
	}
	return totalMana
}

func (p *Player) PlaySpell() {
	for i, c := range p.Hand {
		// check if spell can be cast
		if strings.Contains(c.TypeLine, "Land") {
			continue
		}

		// check if mana available
		err := p.tapForMana(int(c.CMC))
		if err != nil {
			continue
		}

		fmt.Println("Casting spell: ")
		c.Display()
		// pops card from hand
		_, p.Hand = sliceGet(p.Hand, i)
		c.Cast(nil, p)

	}
}

func (p *Player) tapForMana(manaCost int) error {
	// tap lands for mana
	if int(manaCost) > p.ManaAvailable() {
		return fmt.Errorf("not enough mana available")
	}

	for i := range p.Lands {
		if !p.Lands[i].tapped {
			p.Lands[i].tap()
			manaCost--
		}
		if manaCost == 0 {
			return nil
		}
		if manaCost < 0 {
			return fmt.Errorf("mana cost is now negative")
		}
	}
	for i, c := range p.Creatures {
		if !c.tapped && !c.summoningSickness && c.manaProducer {
			p.Creatures[i].tap()
			manaCost--
		}
		if manaCost == 0 {
			return nil
		}
		if manaCost < 0 {
			return fmt.Errorf("mana cost is now negative")
		}
	}
	for i, a := range p.Artifacts {
		if !a.tapped {
			p.Artifacts[i].tap()
			manaCost--
		}
		if manaCost == 0 {
			return nil
		}
		if manaCost < 0 {
			return fmt.Errorf("mana cost is now negative")
		}
	}
	return nil
}
func (p *Player) Display() {
	fmt.Printf("Player: %s\n", p.Name)
	fmt.Printf("Life: %d\n", p.LifeTotal)
	fmt.Printf("Hand: \n")
	DisplayCards(p.Hand)
	fmt.Printf("Board: \n")
	DisplayPermanants(p.Creatures)
	DisplayPermanants(p.Lands)
}

func (p *Player) EndStep() {
	for i := range p.Creatures {
		p.Creatures[i].damage_counters = 0
	}
}
