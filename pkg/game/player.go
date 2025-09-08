package game

import (
	"errors"
)

// Player represents a player in the game and owns zones.
type Player struct {
	name string
	life int
	lost bool

	Library     []SimpleCard
	Hand        []SimpleCard
	Battlefield []*Permanent
	Graveyard   []SimpleCard
	Exile       []SimpleCard

	manaPool map[ManaType]int
}

func NewPlayer(name string, startingLife int) *Player {
	return &Player{
		name:        name,
		life:        startingLife,
		lost:        false,
		Library:     []SimpleCard{},
		Hand:        []SimpleCard{},
		Battlefield: []*Permanent{},
		Graveyard:   []SimpleCard{},
		Exile:       []SimpleCard{},
		manaPool:    map[ManaType]int{},
	}
}

func (p *Player) GetName() string       { return p.name }
func (p *Player) GetLifeTotal() int     { return p.life }
func (p *Player) SetLifeTotal(life int) { p.life = life }
func (p *Player) HasLost() bool         { return p.lost }

func (p *Player) GetHand() []SimpleCard      { return p.Hand }
func (p *Player) AddCardToHand(c SimpleCard) { p.Hand = append(p.Hand, c) }

// Draw draws n cards from library to hand (if available).
func (p *Player) Draw(n int) int {
	drawn := 0
	for i := 0; i < n && len(p.Library) > 0; i++ {
		top := p.Library[0]
		p.Library = p.Library[1:]
		p.Hand = append(p.Hand, top)
		drawn++
	}
	return drawn
}

// FindCardInHand returns index of a card by name in hand.
func (p *Player) FindCardInHand(name string) int {
	for i, c := range p.Hand {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// PlayLand moves a land card from hand to battlefield. Returns error on failure.
func (p *Player) PlayLand(name string) (*Permanent, error) {
	idx := p.FindCardInHand(name)
	if idx < 0 {
		return nil, errors.New("card not in hand")
	}
	c := p.Hand[idx]
	if !c.IsLand() {
		return nil, errors.New("card is not a land")
	}
	// remove from hand
	p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
	perm := NewPermanent(c, p, p)
	p.Battlefield = append(p.Battlefield, perm)
	return perm, nil
}

// SummonCreature moves a creature card from hand to battlefield as a permanent (no costs enforced here).
func (p *Player) SummonCreature(name string) (*Permanent, error) {
	idx := p.FindCardInHand(name)
	if idx < 0 {
		return nil, errors.New("card not in hand")
	}
	c := p.Hand[idx]
	if !c.IsCreature() {
		return nil, errors.New("card is not a creature")
	}
	p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
	perm := NewPermanent(c, p, p)
	p.Battlefield = append(p.Battlefield, perm)
	return perm, nil
}

// CastPermanent moves a nonland permanent card (artifact/enchantment/planeswalker/etc.) from hand to battlefield.
func (p *Player) CastPermanent(name string) (*Permanent, error) {
	idx := p.FindCardInHand(name)
	if idx < 0 {
		return nil, errors.New("card not in hand")
	}
	c := p.Hand[idx]
	if c.IsLand() || c.IsInstant() || c.IsSorcery() {
		return nil, errors.New("card is not a nonland permanent")
	}
	// remove from hand and create permanent
	p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
	perm := NewPermanent(c, p, p)
	p.Battlefield = append(p.Battlefield, perm)
	return perm, nil
}

// DestroyPermanent moves a permanent to its owner's graveyard.
func (p *Player) DestroyPermanent(perm *Permanent) bool {
	// find on battlefield
	for i, bp := range p.Battlefield {
		if bp == perm {
			// remove
			p.Battlefield = append(p.Battlefield[:i], p.Battlefield[i+1:]...)
			// to graveyard using source card
			p.Graveyard = append(p.Graveyard, perm.source)
			return true
		}
	}
	return false
}

// DestroyPermanentToExile moves a permanent to its owner's exile.
func (p *Player) DestroyPermanentToExile(perm *Permanent) bool {
	for i, bp := range p.Battlefield {
		if bp == perm {
			p.Battlefield = append(p.Battlefield[:i], p.Battlefield[i+1:]...)
			p.Exile = append(p.Exile, perm.source)
			return true
		}
	}
	return false
}

// ExileFromGraveyard moves the most recent matching card from graveyard to exile.
func (p *Player) ExileFromGraveyard(name string) bool {
	for i := len(p.Graveyard) - 1; i >= 0; i-- {
		if p.Graveyard[i].Name == name || name == "" {
			c := p.Graveyard[i]
			p.Graveyard = append(p.Graveyard[:i], p.Graveyard[i+1:]...)
			p.Exile = append(p.Exile, c)
			return true
		}
	}
	return false
}

// GetCreatures returns creature permanents you control.
func (p *Player) GetCreatures() []*Permanent {
	out := []*Permanent{}
	for _, perm := range p.Battlefield {
		if perm.IsCreature() {
			out = append(out, perm)
		}
	}
	return out
}

// GetLands returns land permanents you control.
func (p *Player) GetLands() []*Permanent {
	out := []*Permanent{}
	for _, perm := range p.Battlefield {
		if perm.IsLand() {
			out = append(out, perm)
		}
	}
	return out
}

// Mana pool basics (detailed payment in Task 2)
func (p *Player) GetManaPool() map[ManaType]int { return p.manaPool }
func (p *Player) AddManaToPool(mt ManaType, n int) {
	if n <= 0 {
		return
	}
	if p.manaPool == nil {
		p.manaPool = map[ManaType]int{}
	}
	p.manaPool[mt] = p.manaPool[mt] + n
}
