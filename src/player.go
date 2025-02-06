package main

import (
	"fmt"
	"strings"
)

type manaPool struct {
	w int
	u int
	b int
	r int
	g int
	c int
}

type Player struct {
	Name      string
	LifeTotal int
	Deck      Deck
	Hand      []Card
	Graveyard []Card
	Exile     []Card
	Board     []Permanant
	mana      manaPool
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
		Hand:      []Card{},
		Graveyard: []Card{},
		Exile:     []Card{},
		Board:     []Permanant{},
		mana:      manaPool{},
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
		for i := range p.Board {
			p.Board[i].tapped = false
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
	case "Declare Blockers Step":
	case "Combat Damage Step":
	case "End of Combat Step":
	case "End Step":
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

func (p *Player) PlayLand(t *turn) {
	// if len(p.Hand) <= 0 {
	// 	return
	// }

	for i, c := range p.Hand {
		if t.landPerTurn <= 0 {
			return
		}
		if strings.Contains(c.TypeLine, "Land") {
			_, p.Hand = sliceGet(p.Hand, i)
			fmt.Println("Playing land: ")
			c.Display()

			// adds land to board
			p.Board = append(p.Board, Permanant{
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
	for _, c := range p.Board {
		if c.manaProducer && !c.tapped {
			totalMana++
		}
	}
	return totalMana
}

func (p *Player) PlaySpell() {
	if len(p.Hand) <= 0 {
		return
	}

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
		return fmt.Errorf("Not enough mana available")
	}

	for i := range p.Board {
		if p.Board[i].manaProducer && !p.Board[i].tapped {
			p.Board[i].tap()
			manaCost--
		}
		if manaCost == 0 {
			break
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
	DisplayPermanants(p.Board)
}
