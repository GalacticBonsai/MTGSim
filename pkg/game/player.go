package game

import (
	"errors"
)

// Player represents a player in the game and owns zones.
type Player struct {
	name string
	life int
	lost bool

	// lossReason tracks why this player lost (e.g., life_loss, commander_damage, mill, effect)
	lossReason string

	Library     []SimpleCard
	Hand        []SimpleCard
	Battlefield []*Permanent
	Graveyard   []SimpleCard
	Exile       []SimpleCard
	CommandZone []SimpleCard

	manaPool *ManaPool

	// Commander bookkeeping (CR 903). Names of cards designated as this
	// player's commander(s); cast count from the command zone (for tax,
	// CR 903.8); and damage received per opposing commander, keyed by
	// "<ownerName>|<commanderName>" (CR 704.5u).
	commanderNames          map[string]bool
	commanderCastCount      map[string]int
	commanderDamageReceived map[string]int

	additionalLands int
}

func NewPlayer(name string, startingLife int) *Player {
	return &Player{
		name:                    name,
		life:                    startingLife,
		lost:                    false,
		Library:                 []SimpleCard{},
		Hand:                    []SimpleCard{},
		Battlefield:             []*Permanent{},
		Graveyard:               []SimpleCard{},
		Exile:                   []SimpleCard{},
		CommandZone:             []SimpleCard{},
		manaPool:                NewManaPool(),
		commanderNames:          map[string]bool{},
		commanderCastCount:      map[string]int{},
		commanderDamageReceived: map[string]int{},
	}
}

func (p *Player) GetName() string       { return p.name }
func (p *Player) GetLifeTotal() int     { return p.life }
func (p *Player) SetLifeTotal(life int) { p.life = life }
func (p *Player) HasLost() bool         { return p.lost }
func (p *Player) GetLossReason() string { return p.lossReason }
func (p *Player) SetLossReason(r string) { p.lossReason = r }

// Lose marks the player as lost with the given reason and exiles all of their zones.
func (p *Player) Lose(reason string) {
	if p.lost {
		return
	}
	p.lost = true
	if reason != "" {
		p.lossReason = reason
	}
	p.exileAllZones()
}

// exileAllZones moves all cards/permanents from Battlefield, Hand, Library,
// Graveyard and CommandZone into Exile. Called automatically by Lose.
func (p *Player) exileAllZones() {
	for _, perm := range p.Battlefield {
		p.Exile = append(p.Exile, perm.GetSource())
	}
	p.Battlefield = p.Battlefield[:0]

	p.Exile = append(p.Exile, p.Hand...)
	p.Hand = p.Hand[:0]

	p.Exile = append(p.Exile, p.Library...)
	p.Library = p.Library[:0]

	p.Exile = append(p.Exile, p.Graveyard...)
	p.Graveyard = p.Graveyard[:0]

	p.Exile = append(p.Exile, p.CommandZone...)
	p.CommandZone = p.CommandZone[:0]
}

// DeckedOut returns true if the player's library is empty and they would lose from attempting to draw.
func (p *Player) DeckedOut() bool { return len(p.Library) == 0 }

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

// ReturnPermanentToHand moves a permanent to its owner's hand.
func (p *Player) ReturnPermanentToHand(perm *Permanent) bool {
	for i, bp := range p.Battlefield {
		if bp == perm {
			// remove from battlefield
			p.Battlefield = append(p.Battlefield[:i], p.Battlefield[i+1:]...)
			// to hand using source card
			p.Hand = append(p.Hand, perm.source)
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

// GetLands returns land permanents controlled by the player.
func (p *Player) GetLands() []*Permanent {
	out := []*Permanent{}
	for _, perm := range p.Battlefield {
		if perm.IsLand() {
			out = append(out, perm)
		}
	}
	return out
}

// CanPayForCard checks if the player can pay the mana cost of the given card,
// prioritizing cheaper alternate costs if available.
func (p *Player) CanPayForCard(c SimpleCard) bool {
	if !c.HasAlternateCosts() {
		cost := c.GetManaCost()
		return p.manaPool.CanPay(cost)
	}
	// For cards with alternate costs, check if we can pay ANY of them
	// This allows the player to pay the most efficient cost available
	costs := c.GetAlternateCosts()
	for _, cost := range costs {
		if p.manaPool.CanPay(cost) {
			return true
		}
	}
	return false
}

// PayForCard pays the mana cost of the given card if possible,
// prioritizing cheaper alternate costs if available.
func (p *Player) PayForCard(c SimpleCard) bool {
	if !c.HasAlternateCosts() {
		cost := c.GetManaCost()
		return p.manaPool.Pay(cost)
	}
	// For cards with alternate costs, try to pay the minimum cost first
	// Then try other costs in order of ascending cost
	costs := c.GetAlternateCosts()
	type costWithTotal struct {
		mana  Mana
		total int
	}
	var sortedCosts []costWithTotal
	for _, cost := range costs {
		sortedCosts = append(sortedCosts, costWithTotal{cost, cost.Total()})
	}
	// Simple sort by total cost (insertion sort is fine for small lists)
	for i := 1; i < len(sortedCosts); i++ {
		for j := i; j > 0 && sortedCosts[j].total < sortedCosts[j-1].total; j-- {
			sortedCosts[j], sortedCosts[j-1] = sortedCosts[j-1], sortedCosts[j]
		}
	}
	// Try paying from cheapest to most expensive
	for _, ct := range sortedCosts {
		if p.manaPool.Pay(ct.mana) {
			return true
		}
	}
	return false
}

// CanPayForCommander checks the printed commander cost plus commander tax.
func (p *Player) CanPayForCommander(c SimpleCard) bool {
	cost := c.GetManaCost()
	cost.Add(Any, p.CommanderTax(c.Name))
	return p.manaPool.CanPay(cost)
}

// PayForCommander pays the printed commander cost plus commander tax.
func (p *Player) PayForCommander(c SimpleCard) bool {
	cost := c.GetManaCost()
	cost.Add(Any, p.CommanderTax(c.Name))
	return p.manaPool.Pay(cost)
}

// Discard moves n cards from hand to graveyard.
func (p *Player) Discard(n int) []SimpleCard {
	if n > len(p.Hand) {
		n = len(p.Hand)
	}
	discarded := make([]SimpleCard, n)
	copy(discarded, p.Hand[:n])
	p.Hand = p.Hand[n:]
	p.Graveyard = append(p.Graveyard, discarded...)
	return discarded
}

// SearchLibraryToHand moves up to n cards from library to hand.
func (p *Player) SearchLibraryToHand(n int) []SimpleCard {
	if n > len(p.Library) {
		n = len(p.Library)
	}
	found := make([]SimpleCard, n)
	copy(found, p.Library[:n])
	p.Library = p.Library[n:]
	p.Hand = append(p.Hand, found...)
	return found
}

// PutTokenOnBattlefield creates a token permanent and adds it to the battlefield.
func (p *Player) PutTokenOnBattlefield(token SimpleCard) *Permanent {
	perm := NewPermanent(token, p, p)
	p.Battlefield = append(p.Battlefield, perm)
	return perm
}

// Mana pool basics (detailed payment in Task 2)
func (p *Player) GetManaPool() map[ManaType]int { return p.manaPool.pool }
func (p *Player) AddManaToPool(mt ManaType, n int) {
	if n <= 0 {
		return
	}
	p.manaPool.Add(mt, n)
}

func (p *Player) ClearManaPool() { p.manaPool.Clear() }

func (p *Player) AddLandPlay(n int) { p.additionalLands += n }

func (p *Player) LandPlaysAvailable() int {
	return 1 + p.additionalLands
}

func (p *Player) UseLandPlay() {
	if p.additionalLands > 0 {
		p.additionalLands--
	} else {
		p.additionalLands = -1
	}
}

func (p *Player) ResetLandPlays() { p.additionalLands = 0 }
