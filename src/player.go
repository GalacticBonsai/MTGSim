package main

import "fmt"

type manaPool struct {
	w int
	u int
	b int
	r int
	g int
	c int
}

type Player struct {
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
		if len(p.Hand) > 7 {
			// discard down to 7
			p.Hand = p.Hand[:7]
		}
	}
}

func (p *Player) PlayLand(t *turn) {
	if t.landPerTurn <= 0 {
		return
	}

	if len(p.Hand) <= 0 {
		return
	}

	for i, c := range p.Hand {
		if c.TypeLine == "Land" {
			land := p.Hand[i]
			fmt.Println("Playing land: ")
			land.Display()

			// adds land to board
			p.Board = append(p.Board, Permanant{
				source:            land,
				owner:             p,
				tokenType:         Land,
				manaProducer:      true,
				tapped:            false,
				summoningSickness: false,
			})

			// pops card from hand
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
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
		if c.TypeLine == "Land" {
			continue
		}
		
		// check if mana available
		err := p.tapForMana(int(c.CMC))
		if err != nil {
			return
		}
		fmt.Println("Casting spell: ")
		c.Display()
		// pops card from hand
		p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)

		c.Cast(nil, p.Opponents[0])

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
	}
	return nil
}
