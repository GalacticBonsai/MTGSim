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
	deck, _, err := importDeckfile(decklist)
	if err != nil {
		// handle the error appropriately, e.g., log it or return it
		panic(err)
	}
	return &Player{
		Name:      deck.Name,
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
			if !CardHasEvergreenAbility(p.Creatures[i].source, "Haste") {
				p.Creatures[i].summoningSickness = false
			}
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

func (p *Player) DeclareBlockers() {
	for i, creature := range p.Creatures {
		if creature.tapped {
			continue
		}
		for j, attacker := range p.Opponents[0].Creatures {
			if attacker.attacking == p && !attacker.blocked {
				if CanBlock(attacker, creature) {
					p.Creatures[i].blocking = &p.Opponents[0].Creatures[j]
					p.Opponents[0].Creatures[j].blocked = true
					LogPlayer("%s blocked by %s", attacker.source.Name, creature.source.Name)
					break // exit out to not block all attackers
				}
			}
		}
	}
}

func (p *Player) DeclareAttackers() {
	LogPlayer("Declare attacker:")
	attacking := false
	for i, creature := range p.Creatures {
		if creature.tapped || (creature.summoningSickness && !CardHasEvergreenAbility(creature.source, "Haste") || CardHasEvergreenAbility(creature.source, "Defender")) {
			continue
		}

		creature.Display()
		p.Creatures[i].attacking = p.Opponents[0]
		attacking = true

		// Handle Vigilance: Attacking doesn't cause this creature to tap
		if !CardHasEvergreenAbility(creature.source, "Vigilance") {
			creature.tap()
		} else {
			LogPlayer("%s attacks with Vigilance and does not tap.", creature.source.Name)
		}
	}
	if !attacking {
		LogPlayer("No Attackers declared.")
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

func (p *Player) ManaAvailable() *ManaPool {
	manaPool := NewManaPool()

	for _, land := range p.Lands {
		if !land.tapped && land.manaProducer {
			for _, manaType := range land.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	for _, creature := range p.Creatures {
		if !creature.tapped && creature.manaProducer && !creature.summoningSickness {
			for _, manaType := range creature.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	for _, artifact := range p.Artifacts {
		if !artifact.tapped && artifact.manaProducer {
			for _, manaType := range artifact.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	return manaPool
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

func (p *Player) CastSpell(card *Card, target *Permanant) {
	// Handle Ward
	if target != nil && CardHasEvergreenAbility(target.source, "Ward") {
		LogPlayer("%s has Ward. The spell is countered unless the opponent pays the Ward cost.", target.source.Name)
		// For now, assume the opponent cannot pay the cost
		LogPlayer("The opponent cannot pay the Ward cost. The spell is countered.")
		p.Graveyard = append(p.Graveyard, *card)
		return
	}

	// Handle Flash (allow casting as instant, not implemented here, but could be checked in game flow)
	if CardHasEvergreenAbility(*card, "Flash") {
		LogPlayer("%s has Flash and can be cast as an instant.", card.Name)
		// Actual timing handled in game flow
	}

	// Handle Equip
	if strings.Contains(card.TypeLine, "Equipment") && CardHasEvergreenAbility(*card, "Equip") {
		LogPlayer("Equipping %s to a target creature.", card.Name)
		if target != nil && target.tokenType == Creature {
			LogPlayer("%s is now equipped to %s.", card.Name, target.source.Name)
			// Attach Equipment: add card to target's equipment list (if you have one)
		} else {
			LogPlayer("No valid target for Equip. The spell fails.")
		}
		return
	}

	// Handle Enchant
	if strings.Contains(card.TypeLine, "Aura") && CardHasEvergreenAbility(*card, "Enchant") {
		if target != nil {
			LogPlayer("Enchanting %s with %s.", target.source.Name, card.Name)
			// Attach Aura: add card to target's aura list (if you have one)
		} else {
			LogPlayer("No valid target for Enchant. The spell fails.")
		}
		return
	}

	// Handle Scry
	if CardHasEvergreenAbility(*card, "Scry") {
		LogPlayer("Scrying with %s.", card.Name)
		p.Scry(2) // Example: Scry 2
		return
	}

	// Handle Mill
	if CardHasEvergreenAbility(*card, "Mill") {
		LogPlayer("Milling with %s.", card.Name)
		p.Opponents[0].Mill(3) // Example: Mill 3
		return
	}

	// Handle Fight
	if CardHasEvergreenAbility(*card, "Fight") {
		LogPlayer("Fighting with %s.", card.Name)
		if target != nil && target.tokenType == Creature {
			LogPlayer("%s fights %s.", card.Name, target.source.Name)
			p.Fight(card, target)
		} else {
			LogPlayer("No valid target for Fight. The spell fails.")
		}
		return
	}

	// Handle Goad
	if CardHasEvergreenAbility(*card, "Goad") {
		LogPlayer("Goading with %s.", card.Name)
		if target != nil && target.tokenType == Creature {
			LogPlayer("%s is goaded and must attack next turn.", target.source.Name)
			target.goaded = true // You may need to add this field to Permanant
		} else {
			LogPlayer("No valid target for Goad. The spell fails.")
		}
		return
	}

	// Default spell casting logic
	card.Cast(target, p)
}

// Scry: Look at the top X cards of your library, reorder or put any on the bottom
func (p *Player) Scry(n int) {
	if n <= 0 || len(p.Deck.Cards) == 0 {
		return
	}
	peek := n
	if peek > len(p.Deck.Cards) {
		peek = len(p.Deck.Cards)
	}
	top := p.Deck.Cards[:peek]
	LogPlayer("Scry: Top %d cards: %v", peek, top)
	// For now, just leave them in order (no reordering UI)
}

// Mill: Put the top X cards of your library into your graveyard
func (p *Player) Mill(n int) {
	if n <= 0 || len(p.Deck.Cards) == 0 {
		return
	}
	mill := n
	if mill > len(p.Deck.Cards) {
		mill = len(p.Deck.Cards)
	}
	milled := p.Deck.Cards[:mill]
	p.Deck.Cards = p.Deck.Cards[mill:]
	p.Graveyard = append(p.Graveyard, milled...)
	LogPlayer("Milled %d cards: %v", mill, milled)
}

// Fight: Two creatures deal damage equal to their power to each other
func (p *Player) Fight(card *Card, target *Permanant) {
	// Find the source creature (the one that cast the fight spell)
	var source *Permanant
	for i := range p.Creatures {
		if p.Creatures[i].source.Name == card.Name {
			source = &p.Creatures[i]
			break
		}
	}
	if source == nil {
		LogPlayer("Fight source creature not found on battlefield.")
		return
	}
	LogPlayer("%s and %s fight!", source.source.Name, target.source.Name)
	target.damage_counters += source.power
	source.damage_counters += target.power
	source.checkLife()
	target.checkLife()
}

func (p *Player) tapForMana(cost mana) error {
	manaPool := p.ManaAvailable()

	if !manaPool.CanPay(cost) {
		return fmt.Errorf("not enough mana available to pay the cost")
	}

	// Create a copy of the cost to avoid modifying the original during iteration
	remainingCost := newMana()
	for manaType, amount := range cost.pool {
		remainingCost.pool[manaType] = amount
	}

	// Helper function to tap permanents for specific mana
	tapForSpecificMana := func(permanents []Permanant, manaType ManaType) {
		for i := range permanents {
			if !permanents[i].tapped && permanents[i].manaProducer {
				for _, producedMana := range permanents[i].manaTypes {
					if producedMana == manaType && remainingCost.pool[manaType] > 0 {
						permanents[i].tap()
						remainingCost.pool[manaType]--
						break
					}
				}
			}
		}
	}

	// Tap lands for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Lands, manaType)
	}

	// Tap creatures for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Creatures, manaType)
	}

	// Tap artifacts for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Artifacts, manaType)
	}

	// Ensure all costs are paid
	if remainingCost.total() > 0 {
		return fmt.Errorf("not enough mana available to pay the cost")
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
